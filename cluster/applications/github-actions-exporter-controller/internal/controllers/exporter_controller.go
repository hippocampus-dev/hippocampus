package controllers

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"time"

	gaeV1 "github-actions-exporter-controller/api/v1"

	"github.com/go-logr/logr"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/xerrors"
	appsV1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
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
	expiresAtAnnotation    = "github-actions-exporter.kaidotio.github.io/expiresAt"
)

type ExporterReconciler struct {
	client.Client
	Log                     logr.Logger
	Scheme                  *runtime.Scheme
	Recorder                record.EventRecorder
	ExporterImage           string
	GitHubAppClientId       string
	GitHubAppInstallationId string
	GitHubAppPrivateKey     string
}

func (r *ExporterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var requeueAfter time.Duration

	exporter := &gaeV1.Exporter{}
	logger := r.Log.WithValues("exporter", req.NamespacedName)
	if err := r.Get(ctx, req.NamespacedName, exporter); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if exporter.Spec.TokenSecretKeyRef == nil && r.GitHubAppClientId != "" && r.GitHubAppInstallationId != "" && r.GitHubAppPrivateKey != "" {
		var tokenSecret v1.Secret
		if err := r.Client.Get(
			ctx,
			client.ObjectKey{
				Name:      req.Name,
				Namespace: req.Namespace,
			},
			&tokenSecret,
		); apierrors.IsNotFound(err) {
			tokenSecret, err := r.createTokenSecret(ctx, exporter)
			if err != nil {
				return ctrl.Result{}, err
			}
			if err := controllerutil.SetControllerReference(exporter, tokenSecret, r.Scheme); err != nil {
				return ctrl.Result{}, err
			}
			if err := r.Create(ctx, tokenSecret); err != nil {
				return ctrl.Result{}, err
			}
			r.Recorder.Eventf(exporter, v1.EventTypeNormal, "SuccessfulCreated", "Created token secret: %q", tokenSecret.Name)

			expire, err := time.Parse(time.RFC3339, tokenSecret.Annotations[expiresAtAnnotation])
			if err != nil {
				return ctrl.Result{}, err
			}
			requeueAfter = expire.Sub(time.Now()) - time.Minute
		} else if err != nil {
			return ctrl.Result{}, err
		} else {
			expectedTokenSecret, err := r.createTokenSecret(ctx, exporter)
			if err != nil {
				return ctrl.Result{}, err
			}
			if !reflect.DeepEqual(tokenSecret.Data, expectedTokenSecret.Data) ||
				!reflect.DeepEqual(tokenSecret.StringData, expectedTokenSecret.StringData) {
				tokenSecret.Annotations = expectedTokenSecret.Annotations
				tokenSecret.Data = expectedTokenSecret.Data
				tokenSecret.StringData = expectedTokenSecret.StringData

				if err := r.Update(ctx, &tokenSecret); err != nil {
					return ctrl.Result{}, err
				}
				r.Recorder.Eventf(exporter, v1.EventTypeNormal, "SuccessfulUpdated", "Updated token secret: %q", tokenSecret.Name)

				expire, err := time.Parse(time.RFC3339, tokenSecret.Annotations[expiresAtAnnotation])
				if err != nil {
					return ctrl.Result{}, err
				}
				logger.V(1).Info("reconcile", "tokenSecret", tokenSecret.Name, "expiresAt", expire.String())
				requeueAfter = expire.Sub(time.Now()) - time.Minute
			}
		}

		exporter.Spec.TokenSecretKeyRef = &v1.SecretKeySelector{
			LocalObjectReference: v1.LocalObjectReference{
				Name: req.Name,
			},
			Key: "GITHUB_TOKEN",
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
		deployment = *r.buildDeployment(exporter)
		if err := controllerutil.SetControllerReference(exporter, &deployment, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}
		if err := r.Create(ctx, &deployment); err != nil {
			return ctrl.Result{}, err
		}
		r.Recorder.Eventf(exporter, v1.EventTypeNormal, "SuccessfulCreated", "Created deployment: %q", deployment.Name)
	} else if err != nil {
		return ctrl.Result{}, err
	} else {
		expectedDeployment := r.buildDeployment(exporter)
		if !reflect.DeepEqual(deployment.Spec.Template, expectedDeployment.Spec.Template) {
			deployment.Spec.Template = expectedDeployment.Spec.Template

			if err := r.Update(ctx, &deployment); err != nil {
				if strings.Contains(err.Error(), optimisticLockErrorMsg) {
					return ctrl.Result{RequeueAfter: time.Second}, nil
				}
				return ctrl.Result{}, err
			}
			r.Recorder.Eventf(exporter, v1.EventTypeNormal, "SuccessfulUpdated", "Updated deployment: %q", deployment.Name)
		}
	}

	var service v1.Service
	if err := r.Client.Get(
		ctx,
		client.ObjectKey{
			Name:      req.Name,
			Namespace: req.Namespace,
		},
		&service,
	); apierrors.IsNotFound(err) {
		service = *r.buildService(exporter)
		if err := controllerutil.SetControllerReference(exporter, &service, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}
		if err := r.Create(ctx, &service); err != nil {
			return ctrl.Result{}, err
		}
		r.Recorder.Eventf(exporter, v1.EventTypeNormal, "SuccessfulCreated", "Created service: %q", service.Name)
	} else if err != nil {
		return ctrl.Result{}, err
	} else {
		expectedService := r.buildService(exporter)
		if !reflect.DeepEqual(service.Spec.Ports, expectedService.Spec.Ports) ||
			!reflect.DeepEqual(service.Spec.Selector, expectedService.Spec.Selector) {
			service.Spec.Ports = expectedService.Spec.Ports
			service.Spec.Selector = expectedService.Spec.Selector

			if err := r.Update(ctx, &service); err != nil {
				if strings.Contains(err.Error(), optimisticLockErrorMsg) {
					return ctrl.Result{RequeueAfter: time.Second}, nil
				}
				return ctrl.Result{}, err
			}
			r.Recorder.Eventf(exporter, v1.EventTypeNormal, "SuccessfulUpdated", "Updated service: %q", service.Name)
		}
	}

	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

func (r *ExporterReconciler) buildExporterContainer(exporter *gaeV1.Exporter) v1.Container {
	var env []v1.EnvVar
	var envFrom []v1.EnvFromSource
	var volumeMounts []v1.VolumeMount

	env = append(env, v1.EnvVar{
		Name:  "GITHUB_OWNER",
		Value: exporter.Spec.Owner,
	})

	if exporter.Spec.Repo != "" {
		env = append(env, v1.EnvVar{
			Name:  "GITHUB_REPO",
			Value: exporter.Spec.Repo,
		})
	}

	if exporter.Spec.TokenSecretKeyRef != nil {
		env = append(env, v1.EnvVar{
			Name:  "GITHUB_TOKEN_FILE",
			Value: "/mnt/secrets/GITHUB_TOKEN",
		})
		volumeMounts = append(volumeMounts, v1.VolumeMount{
			Name:      "token",
			MountPath: "/mnt/secrets",
			ReadOnly:  true,
		})
	}

	if exporter.Spec.AppSecretRef != nil {
		envFrom = append(envFrom, v1.EnvFromSource{
			SecretRef: exporter.Spec.AppSecretRef,
		})
	}

	return v1.Container{
		Name: "exporter",
		SecurityContext: &v1.SecurityContext{
			Privileged:               ptr.To(false),
			AllowPrivilegeEscalation: ptr.To(false),
			Capabilities: &v1.Capabilities{
				Drop: []v1.Capability{"ALL"},
			},
			ReadOnlyRootFilesystem: ptr.To(true),
			RunAsUser:              ptr.To[int64](65532),
			RunAsNonRoot:           ptr.To(true),
			SeccompProfile: &v1.SeccompProfile{
				Type: v1.SeccompProfileTypeRuntimeDefault,
			},
		},
		Image:           r.ExporterImage,
		ImagePullPolicy: v1.PullIfNotPresent,
		EnvFrom:         envFrom,
		Env:             env,
		Resources: v1.ResourceRequirements{
			Requests: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("10m"),
				v1.ResourceMemory: resource.MustParse("16Mi"),
			},
		},
		Ports: []v1.ContainerPort{
			{
				Name:          "metrics",
				ContainerPort: 8080,
				Protocol:      v1.ProtocolTCP,
			},
		},
		ReadinessProbe: &v1.Probe{
			ProbeHandler: v1.ProbeHandler{
				HTTPGet: &v1.HTTPGetAction{
					Path: "/healthz",
					Port: intstr.FromString("metrics"),
				},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       10,
			SuccessThreshold:    1,
			FailureThreshold:    3,
			TimeoutSeconds:      5,
		},
		LivenessProbe: &v1.Probe{
			ProbeHandler: v1.ProbeHandler{
				HTTPGet: &v1.HTTPGetAction{
					Path: "/healthz",
					Port: intstr.FromString("metrics"),
				},
			},
			InitialDelaySeconds: 10,
			PeriodSeconds:       30,
			SuccessThreshold:    1,
			FailureThreshold:    3,
			TimeoutSeconds:      5,
		},
		VolumeMounts:             volumeMounts,
		TerminationMessagePath:   v1.TerminationMessagePathDefault,
		TerminationMessagePolicy: v1.TerminationMessageReadFile,
	}
}

func (r *ExporterReconciler) buildDeployment(exporter *gaeV1.Exporter) *appsV1.Deployment {
	containers := []v1.Container{
		r.buildExporterContainer(exporter),
	}

	appLabel := exporter.Name
	labels := map[string]string{
		"app.kubernetes.io/name": appLabel,
	}
	for k, v := range exporter.Spec.Template.ObjectMeta.Labels {
		labels[k] = v
	}
	exporter.Spec.Template.ObjectMeta.Labels = labels
	annotations := map[string]string{}
	for k, v := range exporter.Spec.Template.ObjectMeta.Annotations {
		annotations[k] = v
	}
	exporter.Spec.Template.ObjectMeta.Annotations = annotations

	var volumes []v1.Volume

	if exporter.Spec.TokenSecretKeyRef != nil {
		volumes = append(volumes, v1.Volume{
			Name: "token",
			VolumeSource: v1.VolumeSource{
				Secret: &v1.SecretVolumeSource{
					SecretName: exporter.Spec.TokenSecretKeyRef.Name,
				},
			},
		})
	}

	return &appsV1.Deployment{
		ObjectMeta: metaV1.ObjectMeta{
			Name:      exporter.Name,
			Namespace: exporter.Namespace,
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
					MaxUnavailable: &intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: 1,
					},
				},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: exporter.Spec.Template.ObjectMeta,
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
					Containers:                    containers,
					Volumes:                       volumes,
					RestartPolicy:                 v1.RestartPolicyAlways,
					TerminationGracePeriodSeconds: ptr.To[int64](30),
					DNSPolicy:                     v1.DNSClusterFirst,
					SecurityContext: &v1.PodSecurityContext{
						SeccompProfile: &v1.SeccompProfile{
							Type: v1.SeccompProfileTypeRuntimeDefault,
						},
					},
					SchedulerName: v1.DefaultSchedulerName,
				},
			},
		},
	}
}

func (r *ExporterReconciler) buildService(exporter *gaeV1.Exporter) *v1.Service {
	return &v1.Service{
		ObjectMeta: metaV1.ObjectMeta{
			Name:      exporter.Name,
			Namespace: exporter.Namespace,
		},
		Spec: v1.ServiceSpec{
			Type: v1.ServiceTypeClusterIP,
			Selector: map[string]string{
				"app.kubernetes.io/name": exporter.Name,
			},
			Ports: []v1.ServicePort{
				{
					Name:       "metrics",
					Port:       8080,
					TargetPort: intstr.FromString("metrics"),
					Protocol:   v1.ProtocolTCP,
				},
			},
		},
	}
}

func (r *ExporterReconciler) createTokenSecret(ctx context.Context, exporter *gaeV1.Exporter) (*v1.Secret, error) {
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

	if exporter.Spec.Repo == "" {
		body.Permissions = map[string]string{
			"actions":  "read",
			"metadata": "read",
		}
	} else {
		body.Repositories = []string{exporter.Spec.Repo}
		body.Permissions = map[string]string{
			"actions":  "read",
			"metadata": "read",
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
			Name:      exporter.Name,
			Namespace: exporter.Namespace,
			Annotations: map[string]string{
				expiresAtAnnotation: accessToken.ExpiresAt,
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

func (r *ExporterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gaeV1.Exporter{}).
		Owns(&v1.Secret{}).
		Owns(&appsV1.Deployment{}).
		Owns(&v1.Service{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		Complete(r)
}
