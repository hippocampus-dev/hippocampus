package controllers

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"time"

	garV1 "github-actions-runner-controller/api/v1"
	"github-actions-runner-controller/internal/image"
	"github-actions-runner-controller/internal/isolation"

	"github.com/go-logr/logr"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/xerrors"
	appsV1 "k8s.io/api/apps/v1"
	coreV1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	optimisticLockErrorMsg = "the object has been modified; please apply your changes to the latest version and try again"
	expiresAtAnnotation    = "github-actions-runner.kaidotio.github.io/expiresAt"
	repoAnnotation         = "github-actions-runner.kaidotio.github.io/repo"
	// Mounted Secret updates reach the Pod only after the kubelet resync period.
	// Nothing rolls the Pod on renewal, so a margin below syncFrequency plus the delay configMapAndSecretChangeDetectionStrategy selects opens a gap, and kubectl get --raw /api/v1/nodes/{node}/proxy/configz reports both running values
	// A shutdown landing in that gap hands config.sh remove an expired token and leaves the runner registered while the pod deletion, the Deployment and the Runner CR all report success
	tokenRefreshMargin = 15 * time.Minute
	runnerLabel        = "github-actions-runner-controller"
	// https://github.com/actions/actions-runner-controller/issues/3330
	selfHostedLabel = "self-hosted"
)

type RunnerReconciler struct {
	client.Client
	Log                     logr.Logger
	Scheme                  *runtime.Scheme
	Recorder                record.EventRecorder
	PushRegistryURL         string
	PullRegistryURL         string
	GitHubAppClientId       string
	GitHubAppInstallationId string
	GitHubAppPrivateKey     string
	KanikoImage             string
	BinaryVersion           string
	RunnerVersion           string
}

func (r *RunnerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var requeueAfter time.Duration

	runner := &garV1.Runner{}
	logger := r.Log.WithValues("runner", req.NamespacedName)
	if err := r.Get(ctx, req.NamespacedName, runner); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if runner.Spec.TokenSecretKeyRef == nil && r.GitHubAppClientId != "" && r.GitHubAppInstallationId != "" && r.GitHubAppPrivateKey != "" {
		var tokenSecret v1.Secret
		if err := r.Client.Get(
			ctx,
			client.ObjectKey{
				Name:      req.Name,
				Namespace: req.Namespace,
			},
			&tokenSecret,
		); apierrors.IsNotFound(err) {
			tokenSecret, err := r.createTokenSecret(ctx, runner)
			if err != nil {
				return ctrl.Result{}, err
			}
			if err := controllerutil.SetControllerReference(runner, tokenSecret, r.Scheme); err != nil {
				return ctrl.Result{}, err
			}
			if err := r.Create(ctx, tokenSecret); err != nil {
				return ctrl.Result{}, err
			}
			r.Recorder.Eventf(runner, coreV1.EventTypeNormal, "SuccessfulCreated", "Created token secret: %q", tokenSecret.Name)

			expire, err := time.Parse(time.RFC3339, tokenSecret.Annotations[expiresAtAnnotation])
			if err != nil {
				return ctrl.Result{}, err
			}
			requeueAfter = expire.Sub(time.Now()) - tokenRefreshMargin
		} else if err != nil {
			return ctrl.Result{}, err
		} else {
			expire, err := time.Parse(time.RFC3339, tokenSecret.Annotations[expiresAtAnnotation])
			// A secret carrying no parsable expiry is refreshed rather than failed, since that refresh is what writes the annotation
			// The scope is compared as well because createTokenSecret takes it from spec.repo and the Deployment carrying the new one rolls in this same reconcile, so a token left on the old scope reaches a pod whose registration-token request answers something other than 201 and ends in the log.Fatalf of bin/runner.go
			if err != nil ||
				expire.Sub(time.Now()) <= tokenRefreshMargin ||
				tokenSecret.Annotations[repoAnnotation] != runner.Spec.Repo ||
				len(tokenSecret.Data["GITHUB_TOKEN"]) == 0 {
				expectedTokenSecret, err := r.createTokenSecret(ctx, runner)
				if err != nil {
					return ctrl.Result{}, err
				}
				tokenSecret.Annotations = expectedTokenSecret.Annotations
				tokenSecret.Data = expectedTokenSecret.Data
				tokenSecret.StringData = expectedTokenSecret.StringData

				if err := r.Update(ctx, &tokenSecret); err != nil {
					return ctrl.Result{}, err
				}
				r.Recorder.Eventf(runner, coreV1.EventTypeNormal, "SuccessfulUpdated", "Updated token secret: %q", tokenSecret.Name)

				expire, err = time.Parse(time.RFC3339, tokenSecret.Annotations[expiresAtAnnotation])
				if err != nil {
					return ctrl.Result{}, err
				}
				logger.V(1).Info("reconcile", "tokenSecret", tokenSecret.Name, "expiresAt", expire.String())
			}
			requeueAfter = expire.Sub(time.Now()) - tokenRefreshMargin
		}

		runner.Spec.TokenSecretKeyRef = &coreV1.SecretKeySelector{
			LocalObjectReference: coreV1.LocalObjectReference{
				Name: req.Name,
			},
			Key: "GITHUB_TOKEN",
		}
	}

	var workspaceConfigMap v1.ConfigMap
	if err := r.Client.Get(
		ctx,
		client.ObjectKey{
			Name:      req.Name,
			Namespace: req.Namespace,
		},
		&workspaceConfigMap,
	); apierrors.IsNotFound(err) {
		workspaceConfigMap = *image.WorkspaceConfigMap(runner.Name, runner.Namespace, runner.Spec.Image, r.BinaryVersion, r.RunnerVersion, runner.Spec.RunnerContainerSpec.Isolation)
		if err := controllerutil.SetControllerReference(runner, &workspaceConfigMap, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}
		if err := r.Create(ctx, &workspaceConfigMap); err != nil {
			return ctrl.Result{}, err
		}
		r.Recorder.Eventf(runner, coreV1.EventTypeNormal, "SuccessfulCreated", "Created workspace config map: %q", workspaceConfigMap.Name)
	} else if err != nil {
		return ctrl.Result{}, err
	} else {
		expectedWorkspaceConfigMap := image.WorkspaceConfigMap(runner.Name, runner.Namespace, runner.Spec.Image, r.BinaryVersion, r.RunnerVersion, runner.Spec.RunnerContainerSpec.Isolation)
		if !reflect.DeepEqual(workspaceConfigMap.Data, expectedWorkspaceConfigMap.Data) ||
			!reflect.DeepEqual(workspaceConfigMap.BinaryData, expectedWorkspaceConfigMap.BinaryData) {
			workspaceConfigMap.Data = expectedWorkspaceConfigMap.Data
			workspaceConfigMap.BinaryData = expectedWorkspaceConfigMap.BinaryData

			if err := r.Update(ctx, &workspaceConfigMap); err != nil {
				return ctrl.Result{}, err
			}
			r.Recorder.Eventf(runner, coreV1.EventTypeNormal, "SuccessfulUpdated", "Updated config map: %q", workspaceConfigMap.Name)
		}
	}

	var deployment appsV1.Deployment
	if err := r.Client.Get(
		ctx,
		client.ObjectKey{
			Name:      req.Name,
			Namespace: req.Namespace,
		},
		&deployment,
	); apierrors.IsNotFound(err) {
		deployment = *r.buildDeployment(runner)
		if err := controllerutil.SetControllerReference(runner, &deployment, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}
		if err := r.Create(ctx, &deployment); err != nil {
			return ctrl.Result{}, err
		}
		r.Recorder.Eventf(runner, coreV1.EventTypeNormal, "SuccessfulCreated", "Created deployment: %q", deployment.Name)
	} else if err != nil {
		return ctrl.Result{}, err
	} else {
		expectedDeployment := r.buildDeployment(runner)
		if !reflect.DeepEqual(deployment.Spec.Template, expectedDeployment.Spec.Template) ||
			!reflect.DeepEqual(deployment.Spec.Strategy, expectedDeployment.Spec.Strategy) {
			deployment.Spec.Template = expectedDeployment.Spec.Template
			deployment.Spec.Strategy = expectedDeployment.Spec.Strategy

			if err := r.Update(ctx, &deployment); err != nil {
				if strings.Contains(err.Error(), optimisticLockErrorMsg) {
					return ctrl.Result{RequeueAfter: time.Second}, nil
				}
				return ctrl.Result{}, err
			}
			r.Recorder.Eventf(runner, coreV1.EventTypeNormal, "SuccessfulUpdated", "Updated deployment: %q", deployment.Name)
		}
	}

	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

func (r *RunnerReconciler) runnerLabels(runner *garV1.Runner) []string {
	labels := []string{runnerLabel}
	for _, name := range runner.Spec.Labels {
		if name == selfHostedLabel || name == runnerLabel {
			continue
		}
		labels = append(labels, name)
	}
	return labels
}

func (r *RunnerReconciler) buildRunnerContainer(runner *garV1.Runner) v1.Container {
	args := []string{
		"--without-install",
		"--hostname=$(HOSTNAME)",
	}
	env := runner.Spec.RunnerContainerSpec.Env
	envFrom := runner.Spec.RunnerContainerSpec.EnvFrom
	volumeMounts := runner.Spec.RunnerContainerSpec.VolumeMounts

	if runner.Spec.Repo == "" {
		args = append(args, "--organization=$(ORGANIZATION)")
		env = append(env, coreV1.EnvVar{
			Name:  "ORGANIZATION",
			Value: runner.Spec.Owner,
		})
	} else {
		args = append(args, "--repository=$(REPOSITORY)")
		env = append(env, coreV1.EnvVar{
			Name:  "REPOSITORY",
			Value: fmt.Sprintf("%s/%s", runner.Spec.Owner, runner.Spec.Repo),
		})
	}

	env = append(env, coreV1.EnvVar{
		Name: "HOSTNAME",
		ValueFrom: &coreV1.EnvVarSource{
			FieldRef: &coreV1.ObjectFieldSelector{
				APIVersion: "v1",
				FieldPath:  "metadata.name",
			},
		},
	})

	if len(runner.Spec.Labels) > 0 {
		args = append(args, "--labels=$(LABELS)")
	}
	env = append(env, coreV1.EnvVar{
		Name:  "LABELS",
		Value: strings.Join(r.runnerLabels(runner), ","),
	})

	if runner.Spec.RunnerGroup != "" {
		args = append(args, "--runner-group=$(RUNNER_GROUP)")
		env = append(env, coreV1.EnvVar{
			Name:  "RUNNER_GROUP",
			Value: runner.Spec.RunnerGroup,
		})
	}

	if runner.Spec.TokenSecretKeyRef != nil {
		args = append(args, "--token=$(TOKEN)")
		env = append(env, coreV1.EnvVar{
			Name:  "TOKEN",
			Value: "/mnt/secrets/GITHUB_TOKEN",
			// Controller updates TokenSecret when issued by GitHub Apps.
			//ValueFrom: &coreV1.EnvVarSource{
			//	SecretKeyRef: runner.Spec.TokenSecretKeyRef,
			//},
		})
		volumeMounts = append(volumeMounts, v1.VolumeMount{
			Name:      "token",
			MountPath: "/mnt/secrets",
			ReadOnly:  true,
		})
	}

	if runner.Spec.AppSecretRef != nil {
		args = append(args, []string{
			"--github-app-id=$(github_app_id)",
			"--github-app-installation-id=$(github_app_installation_id)",
			"--github-app-private-key=$(github_app_private_key)",
		}...)
		envFrom = append(envFrom, coreV1.EnvFromSource{
			SecretRef: runner.Spec.AppSecretRef,
		})
	}

	mode := runner.Spec.RunnerContainerSpec.Isolation

	env = append(env, isolation.StorageEnv(mode)...)
	volumeMounts = append(volumeMounts, isolation.StorageVolumeMounts(mode)...)

	c := v1.Container{
		Name: "runner",
		SecurityContext: &v1.SecurityContext{
			Privileged:               ptr.To(mode == isolation.Privileged),
			AllowPrivilegeEscalation: ptr.To(true),
			// containerd applies neither this nor the seccomp profile to a privileged container, so carrying them there claims a confinement the pod does not have: https://github.com/containerd/containerd/blob/v2.0.0/pkg/oci/spec.go#L118
			Capabilities: func() *v1.Capabilities {
				if mode == isolation.Privileged {
					return nil
				}
				return &v1.Capabilities{
					Add: isolation.Capabilities(mode),
					Drop: []v1.Capability{
						"ALL",
					},
				}
			}(),
			ProcMount:              isolation.ProcMount(mode),
			ReadOnlyRootFilesystem: ptr.To(false),
			RunAsUser:              ptr.To[int64](60000),
			RunAsNonRoot:           ptr.To(true),
			SeccompProfile:         isolation.SeccompProfile(mode),
		},
		Image:                    fmt.Sprintf("%s/%s", r.PullRegistryURL, image.RepositoryName(image.RunnerRepositoryKind, runner.Spec.Image, r.BinaryVersion, r.RunnerVersion, mode)),
		ImagePullPolicy:          v1.PullAlways,
		Args:                     args,
		EnvFrom:                  envFrom,
		Env:                      env,
		Resources:                runner.Spec.RunnerContainerSpec.Resources,
		VolumeMounts:             volumeMounts,
		TerminationMessagePath:   coreV1.TerminationMessagePathDefault,
		TerminationMessagePolicy: coreV1.TerminationMessageReadFile,
	}
	if runner.Spec.Disableupdate {
		c.Args = append(c.Args, "--disableupdate")
	}
	return c
}

func (r *RunnerReconciler) buildDeployment(runner *garV1.Runner) *appsV1.Deployment {
	mode := runner.Spec.RunnerContainerSpec.Isolation

	destination := fmt.Sprintf("%s/%s", r.PushRegistryURL, image.RepositoryName(image.RunnerRepositoryKind, runner.Spec.Image, r.BinaryVersion, r.RunnerVersion, mode))
	cacheRepository := fmt.Sprintf("%s/%s", r.PushRegistryURL, image.CacheRepositoryName)

	containers := []v1.Container{
		r.buildRunnerContainer(runner),
	}

	appLabel := runner.Name
	labels := map[string]string{
		"app.kubernetes.io/name": appLabel,
	}
	for k, v := range runner.Spec.Template.ObjectMeta.Labels {
		labels[k] = v
	}
	runner.Spec.Template.ObjectMeta.Labels = labels
	annotations := map[string]string{
		"image": runner.Spec.Image,
	}
	for k, v := range runner.Spec.Template.ObjectMeta.Annotations {
		annotations[k] = v
	}
	runner.Spec.Template.ObjectMeta.Annotations = annotations

	volumes := runner.Spec.Template.Spec.Volumes

	volumes = append(volumes, isolation.StorageVolumes(mode)...)

	volumes = append(volumes, v1.Volume{
		Name: "workspace",
		VolumeSource: v1.VolumeSource{
			ConfigMap: &v1.ConfigMapVolumeSource{
				LocalObjectReference: v1.LocalObjectReference{
					Name: runner.Name,
				},
			},
		},
	})

	if runner.Spec.TokenSecretKeyRef != nil {
		volumes = append(volumes, v1.Volume{
			Name: "token",
			VolumeSource: v1.VolumeSource{
				Secret: &v1.SecretVolumeSource{
					SecretName: runner.Spec.TokenSecretKeyRef.Name,
				},
			},
		})
	}

	return &appsV1.Deployment{
		ObjectMeta: metaV1.ObjectMeta{
			Name:      runner.Name,
			Namespace: runner.Namespace,
		},
		Spec: appsV1.DeploymentSpec{
			Selector: &metaV1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name": appLabel,
				},
			},
			Replicas: ptr.To[int32](1),
			Strategy: appsV1.DeploymentStrategy{
				Type: appsV1.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &appsV1.RollingUpdateDeployment{
					MaxSurge: &intstr.IntOrString{
						Type:   intstr.String,
						StrVal: "25%",
					},
					// A larger value drops a replica while its kaniko initContainer is still building, and GitHub loses that runner for the length of the build
					MaxUnavailable: &intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: 0,
					},
				},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: runner.Spec.Template.ObjectMeta,
				Spec: v1.PodSpec{
					Affinity: &v1.Affinity{
						PodAntiAffinity: &v1.PodAntiAffinity{
							PreferredDuringSchedulingIgnoredDuringExecution: []v1.WeightedPodAffinityTerm{
								{
									Weight: 100,
									PodAffinityTerm: v1.PodAffinityTerm{
										LabelSelector: &metaV1.LabelSelector{
											MatchLabels: map[string]string{
												"app.kubernetes.io/name": appLabel,
											},
										},
										TopologyKey: "kubernetes.io/hostname",
									},
								},
							},
						},
					},
					ServiceAccountName: runner.Spec.Template.Spec.ServiceAccountName,
					RuntimeClassName:   runner.Spec.Template.Spec.RuntimeClassName,
					HostUsers:          isolation.HostUsers(mode),
					InitContainers: []v1.Container{
						image.BuilderContainer(r.KanikoImage, destination, cacheRepository, runner.Spec.BuilderContainerSpec),
					},
					Containers:                    containers,
					Volumes:                       volumes,
					RestartPolicy:                 coreV1.RestartPolicyAlways,
					TerminationGracePeriodSeconds: ptr.To[int64](90),
					DNSPolicy:                     coreV1.DNSClusterFirst,
					SecurityContext: &coreV1.PodSecurityContext{
						SeccompProfile: &coreV1.SeccompProfile{
							Type: coreV1.SeccompProfileTypeRuntimeDefault,
						},
					},
					SchedulerName: coreV1.DefaultSchedulerName,
				},
			},
		},
	}
}

func (r *RunnerReconciler) createTokenSecret(ctx context.Context, runner *garV1.Runner) (*v1.Secret, error) {
	body := struct {
		Repositories  []string          `json:"repositories,omitempty"`
		RepositoryIds []int             `json:"repository_ids,omitempty"`
		Permissions   map[string]string `json:"permissions"`
	}{}

	accessToken := struct {
		Token     string `json:"token"`
		ExpiresAt string `json:"expires_at"`
	}{}

	err, jwtToken := signJwt(r.GitHubAppPrivateKey, r.GitHubAppClientId)
	if err != nil {
		return nil, xerrors.Errorf("failed to sign jwt: %w", err)
	}

	if runner.Spec.Repo == "" {
		body.Permissions = map[string]string{
			"organization_self_hosted_runners": "write",
			"metadata":                         "read",
		}
	} else {
		body.Repositories = []string{runner.Spec.Repo}
		body.Permissions = map[string]string{
			"administration": "write",
			"metadata":       "read",
		}
	}
	b, err := json.Marshal(body)
	if err != nil {
		return nil, xerrors.Errorf("failed to marshal body: %w", err)
	}

	accessTokenRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("https://api.github.com/app/installations/%s/access_tokens", r.GitHubAppInstallationId), bytes.NewReader(b))
	if err != nil {
		return nil, xerrors.Errorf("failed to create request: %w", err)
	}

	accessTokenRequest.Header.Set("Accept", "application/vnd.github+json")
	accessTokenRequest.Header.Set("Authorization", fmt.Sprintf("Bearer %s", *jwtToken))
	accessTokenRequest.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	accessTokenResponse, err := http.DefaultClient.Do(accessTokenRequest)
	if err != nil {
		return nil, xerrors.Errorf("failed to do request: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, accessTokenResponse.Body)
		_ = accessTokenResponse.Body.Close()
	}()

	if accessTokenResponse.StatusCode != http.StatusCreated {
		return nil, xerrors.Errorf("failed to get access token: %d", accessTokenResponse.StatusCode)
	}

	if err := json.NewDecoder(accessTokenResponse.Body).Decode(&accessToken); err != nil {
		return nil, xerrors.Errorf("failed to decode access token: %w", err)
	}

	return &v1.Secret{
		ObjectMeta: metaV1.ObjectMeta{
			Name:      runner.Name,
			Namespace: runner.Namespace,
			Annotations: map[string]string{
				expiresAtAnnotation: accessToken.ExpiresAt,
				repoAnnotation:      runner.Spec.Repo,
			},
		},
		StringData: map[string]string{
			"GITHUB_TOKEN": accessToken.Token,
		},
	}, nil
}

func signJwt(privateKey string, clientId string) (error, *string) {
	block, _ := pem.Decode([]byte(privateKey))
	if block == nil {
		return xerrors.New("failed to decode private key"), nil
	}

	rsaPrivateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return xerrors.Errorf("failed to parse private key: %w", err), nil
	}

	now := time.Now()
	claims := jwt.MapClaims{
		"iat": now.Unix(),
		"exp": now.Add(time.Minute * 10).Unix(),
		"iss": clientId,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	jwtToken, err := token.SignedString(rsaPrivateKey)
	if err != nil {
		return xerrors.Errorf("failed to sign token: %w", err), nil
	}
	return nil, &jwtToken
}

func (r *RunnerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&garV1.Runner{}).
		Owns(&v1.ConfigMap{}).
		Owns(&appsV1.Deployment{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: reconcileConcurrency}).
		Complete(r)
}
