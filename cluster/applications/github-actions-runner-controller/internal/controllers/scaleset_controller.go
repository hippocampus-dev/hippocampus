package controllers

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"time"

	garV1 "github-actions-runner-controller/api/v1"
	"github-actions-runner-controller/internal/image"
	"github-actions-runner-controller/internal/isolation"
	"github-actions-runner-controller/internal/runnable"

	"github.com/actions/scaleset"
	"github.com/go-logr/logr"
	"golang.org/x/xerrors"
	batchV1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	scaleSetFinalizer = "github-actions-runner.kaidotio.github.io/finalizer"
	scaleSetUIDLabel  = "github-actions-runner.kaidotio.github.io/scale-set-uid"
	// Nothing enqueues a scale set whose listener stopped, and an idle one produces no job events either.
	listenerRestartInterval = time.Minute
	// https://github.com/actions/actions-runner-controller/blob/master/controllers/actions.github.com/autoscalingrunnerset_controller.go
	scaleSetLabelType = "System"
)

type ScaleSetReconciler struct {
	client.Client
	Log                     logr.Logger
	Scheme                  *runtime.Scheme
	Recorder                record.EventRecorder
	APIReader               client.Reader
	PushRegistryURL         string
	PullRegistryURL         string
	GitHubAppClientId       string
	GitHubAppInstallationId string
	GitHubAppPrivateKey     string
	KanikoImage             string
	BinaryVersion           string
	RunnerVersion           string
	Listener                *runnable.ScaleSetListener
}

func (r *ScaleSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	scaleSet := &garV1.ScaleSet{}
	logger := r.Log.WithValues("scaleset", req.NamespacedName)
	if err := r.Get(ctx, req.NamespacedName, scaleSet); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if !scaleSet.DeletionTimestamp.IsZero() {
		if err := r.finalize(ctx, scaleSet); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(scaleSet, scaleSetFinalizer) {
		controllerutil.AddFinalizer(scaleSet, scaleSetFinalizer)
		if err := r.Update(ctx, scaleSet); err != nil {
			return ctrl.Result{}, err
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
		workspaceConfigMap = *image.WorkspaceConfigMap(scaleSet.Name, scaleSet.Namespace, scaleSet.Spec.Image, r.BinaryVersion, r.RunnerVersion)
		if err := controllerutil.SetControllerReference(scaleSet, &workspaceConfigMap, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}
		if err := r.Create(ctx, &workspaceConfigMap); err != nil {
			return ctrl.Result{}, err
		}
		r.Recorder.Eventf(scaleSet, v1.EventTypeNormal, "SuccessfulCreated", "Created workspace config map: %q", workspaceConfigMap.Name)
	} else if err != nil {
		return ctrl.Result{}, err
	} else {
		expectedWorkspaceConfigMap := image.WorkspaceConfigMap(scaleSet.Name, scaleSet.Namespace, scaleSet.Spec.Image, r.BinaryVersion, r.RunnerVersion)
		if !reflect.DeepEqual(workspaceConfigMap.Data, expectedWorkspaceConfigMap.Data) {
			workspaceConfigMap.Data = expectedWorkspaceConfigMap.Data

			if err := r.Update(ctx, &workspaceConfigMap); err != nil {
				return ctrl.Result{}, err
			}
			r.Recorder.Eventf(scaleSet, v1.EventTypeNormal, "SuccessfulUpdated", "Updated workspace config map: %q", workspaceConfigMap.Name)
		}
	}

	built, err := r.reconcileBuildJob(ctx, scaleSet)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !built {
		return ctrl.Result{}, nil
	}

	// Skipped while the poll loop is current: building a client here would discard its cached admin token and exchange a new one against the REST API on every requeue.
	if !r.Listener.Running(scaleSet.UID, scaleSet.Generation) {
		actionsClient, err := r.newClient(ctx, scaleSet)
		if err != nil {
			return ctrl.Result{}, err
		}

		scaleSetID, err := r.registerScaleSet(ctx, actionsClient, scaleSet)
		if err != nil {
			return ctrl.Result{}, err
		}

		desiredRunners := min(scaleSet.Spec.MaxRunners, scaleSet.Spec.MinRunners)
		if scaleSet.Status.ScaleSetID != scaleSetID || scaleSet.Status.DesiredRunners != desiredRunners {
			scaleSet.Status.ScaleSetID = scaleSetID
			scaleSet.Status.DesiredRunners = desiredRunners
			if err := r.Status().Update(ctx, scaleSet); err != nil {
				return ctrl.Result{}, err
			}
		}

		r.Listener.Ensure(scaleSet, actionsClient, scaleSetID)
	}

	logger.V(1).Info("reconcile", "desiredRunners", scaleSet.Status.DesiredRunners)

	if err := r.reconcileRunners(ctx, scaleSet); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: listenerRestartInterval}, nil
}

func (r *ScaleSetReconciler) buildJobName(scaleSet *garV1.ScaleSet) string {
	return fmt.Sprintf("%s-build-%s", scaleSet.Name, image.RepositoryName(scaleSet.Spec.Image, r.BinaryVersion, r.RunnerVersion))
}

func (r *ScaleSetReconciler) reconcileBuildJob(ctx context.Context, scaleSet *garV1.ScaleSet) (bool, error) {
	var job batchV1.Job
	if err := r.Client.Get(
		ctx,
		client.ObjectKey{
			Name:      r.buildJobName(scaleSet),
			Namespace: scaleSet.Namespace,
		},
		&job,
	); apierrors.IsNotFound(err) {
		job = *r.buildBuildJob(scaleSet)
		if err := controllerutil.SetControllerReference(scaleSet, &job, r.Scheme); err != nil {
			return false, err
		}
		if err := r.Create(ctx, &job); err != nil {
			return false, err
		}
		r.Recorder.Eventf(scaleSet, v1.EventTypeNormal, "SuccessfulCreated", "Created build job: %q", job.Name)
		return false, r.deleteStaleBuildJobs(ctx, scaleSet)
	} else if err != nil {
		return false, err
	}

	return job.Status.Succeeded > 0, nil
}

func (r *ScaleSetReconciler) deleteStaleBuildJobs(ctx context.Context, scaleSet *garV1.ScaleSet) error {
	var jobs batchV1.JobList
	if err := r.List(
		ctx,
		&jobs,
		client.InNamespace(scaleSet.Namespace),
		client.MatchingLabels{scaleSetUIDLabel: string(scaleSet.UID)},
	); err != nil {
		return err
	}

	current := r.buildJobName(scaleSet)
	for i := range jobs.Items {
		job := &jobs.Items[i]
		if job.Labels["app.kubernetes.io/component"] == "runner" || job.Name == current {
			continue
		}
		if err := r.Delete(ctx, job, client.PropagationPolicy(metaV1.DeletePropagationBackground)); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		r.Recorder.Eventf(scaleSet, v1.EventTypeNormal, "SuccessfulDeleted", "Deleted build job: %q", job.Name)
	}

	return nil
}

func (r *ScaleSetReconciler) buildBuildJob(scaleSet *garV1.ScaleSet) *batchV1.Job {
	destination := fmt.Sprintf("%s/%s", r.PushRegistryURL, image.RepositoryName(scaleSet.Spec.Image, r.BinaryVersion, r.RunnerVersion))
	builderContainer := image.BuilderContainer(r.KanikoImage, destination, scaleSet.Spec.BuilderContainerSpec)

	return &batchV1.Job{
		ObjectMeta: metaV1.ObjectMeta{
			Name:      r.buildJobName(scaleSet),
			Namespace: scaleSet.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name": scaleSet.Name,
				scaleSetUIDLabel:         string(scaleSet.UID),
			},
		},
		Spec: batchV1.JobSpec{
			Completions: ptr.To[int32](1),
			Parallelism: ptr.To[int32](1),
			Template: v1.PodTemplateSpec{
				ObjectMeta: metaV1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/name": scaleSet.Name,
						scaleSetUIDLabel:         string(scaleSet.UID),
					},
				},
				Spec: v1.PodSpec{
					ServiceAccountName: scaleSet.Spec.Template.Spec.ServiceAccountName,
					Containers: []v1.Container{
						builderContainer,
					},
					Volumes:       r.buildBuilderVolumes(scaleSet, builderContainer.VolumeMounts),
					RestartPolicy: v1.RestartPolicyNever,
					SecurityContext: &v1.PodSecurityContext{
						SeccompProfile: &v1.SeccompProfile{
							Type: v1.SeccompProfileTypeRuntimeDefault,
						},
					},
				},
			},
		},
	}
}

func (r *ScaleSetReconciler) buildBuilderVolumes(scaleSet *garV1.ScaleSet, volumeMounts []v1.VolumeMount) []v1.Volume {
	return filterMountedVolumes(append(scaleSet.Spec.Template.Spec.Volumes, v1.Volume{
		Name: "workspace",
		VolumeSource: v1.VolumeSource{
			ConfigMap: &v1.ConfigMapVolumeSource{
				LocalObjectReference: v1.LocalObjectReference{
					Name: scaleSet.Name,
				},
			},
		},
	}), volumeMounts)
}

func filterMountedVolumes(volumes []v1.Volume, volumeMounts []v1.VolumeMount) []v1.Volume {
	mountedVolumeNames := make(map[string]struct{}, len(volumeMounts))
	for _, volumeMount := range volumeMounts {
		mountedVolumeNames[volumeMount.Name] = struct{}{}
	}

	filtered := make([]v1.Volume, 0, len(volumes))
	for _, volume := range volumes {
		if _, ok := mountedVolumeNames[volume.Name]; ok {
			filtered = append(filtered, volume)
		}
	}
	return filtered
}

func (r *ScaleSetReconciler) reconcileRunners(ctx context.Context, scaleSet *garV1.ScaleSet) error {
	var jobs batchV1.JobList
	// Reads through the API rather than the cache: a job created in the previous reconcile may not be cached yet, and undercounting creates another one.
	if err := r.APIReader.List(
		ctx,
		&jobs,
		client.InNamespace(scaleSet.Namespace),
		client.MatchingLabels{
			scaleSetUIDLabel:              string(scaleSet.UID),
			"app.kubernetes.io/component": "runner",
		},
	); err != nil {
		return err
	}

	// Covers both the job that just finished and the one whose TTL removed it while the controller was down.
	if err := r.deleteOrphanedJitConfigSecrets(ctx, scaleSet, &jobs); err != nil {
		return err
	}

	active := 0
	for i := range jobs.Items {
		if jobs.Items[i].Status.Succeeded == 0 && jobs.Items[i].Status.Failed == 0 {
			active++
		}
	}

	for i := active; i < scaleSet.Status.DesiredRunners; i++ {
		if err := r.createRunner(ctx, scaleSet); err != nil {
			return err
		}
	}

	return nil
}

func (r *ScaleSetReconciler) deleteOrphanedJitConfigSecrets(ctx context.Context, scaleSet *garV1.ScaleSet, jobs *batchV1.JobList) error {
	live := make(map[string]struct{}, len(jobs.Items))
	for i := range jobs.Items {
		live[jobs.Items[i].Name] = struct{}{}
	}

	var secrets v1.SecretList
	if err := r.APIReader.List(
		ctx,
		&secrets,
		client.InNamespace(scaleSet.Namespace),
		client.MatchingLabels{scaleSetUIDLabel: string(scaleSet.UID)},
	); err != nil {
		return err
	}

	actionsClient, _, running := r.Listener.Session(scaleSet.UID)

	for i := range secrets.Items {
		secret := &secrets.Items[i]
		if _, ok := live[secret.Name]; ok {
			continue
		}
		if !running {
			continue
		}
		runner, err := actionsClient.GetRunnerByName(ctx, secret.Name)
		if err != nil {
			return xerrors.Errorf("failed to get runner: %w", err)
		}
		if runner != nil {
			if err := actionsClient.RemoveRunner(ctx, int64(runner.ID)); err != nil {
				return xerrors.Errorf("failed to remove runner: %w", err)
			}
		}
		if err := r.deleteJitConfigSecret(ctx, scaleSet.Namespace, secret.Name); err != nil {
			return err
		}
	}

	return nil
}

func (r *ScaleSetReconciler) createRunner(ctx context.Context, scaleSet *garV1.ScaleSet) error {
	actionsClient, scaleSetID, running := r.Listener.Session(scaleSet.UID)
	if !running {
		return xerrors.Errorf("listener for %s/%s is not running", scaleSet.Namespace, scaleSet.Name)
	}

	name := fmt.Sprintf("%s-%s", scaleSet.Name, utilrand.String(5))

	// A JIT config is single-use, so it is issued per creation attempt and never reused.
	jitRunnerConfig, err := actionsClient.GenerateJitRunnerConfig(
		ctx,
		&scaleset.RunnerScaleSetJitRunnerSetting{
			Name: name,
		},
		scaleSetID,
	)
	if err != nil {
		return xerrors.Errorf("failed to generate jit runner config: %w", err)
	}

	secret := &v1.Secret{
		ObjectMeta: metaV1.ObjectMeta{
			Name:      name,
			Namespace: scaleSet.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name": scaleSet.Name,
				scaleSetUIDLabel:         string(scaleSet.UID),
			},
		},
		StringData: map[string]string{
			"ACTIONS_RUNNER_INPUT_JITCONFIG": jitRunnerConfig.EncodedJITConfig,
		},
	}
	if err := controllerutil.SetControllerReference(scaleSet, secret, r.Scheme); err != nil {
		return err
	}
	if err := r.Create(ctx, secret); err != nil {
		return err
	}

	job := r.buildRunnerJob(scaleSet, name)
	if err := controllerutil.SetControllerReference(scaleSet, job, r.Scheme); err != nil {
		return err
	}
	if err := r.Create(ctx, job); err != nil {
		return err
	}
	r.Recorder.Eventf(scaleSet, v1.EventTypeNormal, "SuccessfulCreated", "Created runner job: %q", job.Name)

	return nil
}

func (r *ScaleSetReconciler) deleteJitConfigSecret(ctx context.Context, namespace string, name string) error {
	secret := &v1.Secret{
		ObjectMeta: metaV1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	if err := r.Delete(ctx, secret); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

func (r *ScaleSetReconciler) buildRunnerJob(scaleSet *garV1.ScaleSet, name string) *batchV1.Job {
	mode := scaleSet.Spec.RunnerContainerSpec.Isolation

	labels := map[string]string{}
	for k, v := range scaleSet.Spec.Template.ObjectMeta.Labels {
		labels[k] = v
	}
	// Applied last because reconcileRunners counts jobs by these, and a spec label shadowing one makes every reconcile create another runner.
	labels["app.kubernetes.io/name"] = scaleSet.Name
	labels["app.kubernetes.io/component"] = "runner"
	labels[scaleSetUIDLabel] = string(scaleSet.UID)
	runnerContainer := r.buildRunnerContainer(scaleSet, name)

	return &batchV1.Job{
		ObjectMeta: metaV1.ObjectMeta{
			Name:      name,
			Namespace: scaleSet.Namespace,
			Labels:    labels,
		},
		Spec: batchV1.JobSpec{
			// A just-in-time configuration is single-use, so a replacement pod could never register and the job must fail instead.
			BackoffLimit:            ptr.To[int32](0),
			Completions:             ptr.To[int32](1),
			Parallelism:             ptr.To[int32](1),
			TTLSecondsAfterFinished: ptr.To[int32](300),
			Template: v1.PodTemplateSpec{
				ObjectMeta: metaV1.ObjectMeta{
					Labels:      labels,
					Annotations: scaleSet.Spec.Template.ObjectMeta.Annotations,
				},
				Spec: v1.PodSpec{
					ServiceAccountName: scaleSet.Spec.Template.Spec.ServiceAccountName,
					RuntimeClassName:   scaleSet.Spec.Template.Spec.RuntimeClassName,
					HostUsers:          isolation.HostUsers(mode, scaleSet.Spec.Template.Spec.HostUsers),
					Containers: []v1.Container{
						runnerContainer,
					},
					Volumes:       r.buildRunnerVolumes(scaleSet, runnerContainer.VolumeMounts),
					RestartPolicy: v1.RestartPolicyNever,
					SecurityContext: &v1.PodSecurityContext{
						SeccompProfile: &v1.SeccompProfile{
							Type: v1.SeccompProfileTypeRuntimeDefault,
						},
					},
				},
			},
		},
	}
}

func (r *ScaleSetReconciler) buildRunnerContainer(scaleSet *garV1.ScaleSet, name string) v1.Container {
	mode := scaleSet.Spec.RunnerContainerSpec.Isolation

	return v1.Container{
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
			SeccompProfile: func() *v1.SeccompProfile {
				if mode == isolation.Privileged {
					return nil
				}
				return &v1.SeccompProfile{
					Type: v1.SeccompProfileTypeRuntimeDefault,
				}
			}(),
		},
		Image:           fmt.Sprintf("%s/%s", r.PullRegistryURL, image.RepositoryName(scaleSet.Spec.Image, r.BinaryVersion, r.RunnerVersion)),
		ImagePullPolicy: v1.PullAlways,
		Command:         []string{"/home/runner/run.sh"},
		EnvFrom: append(scaleSet.Spec.RunnerContainerSpec.EnvFrom, v1.EnvFromSource{
			SecretRef: &v1.SecretEnvSource{
				LocalObjectReference: v1.LocalObjectReference{
					Name: name,
				},
			},
		}),
		Env: append(scaleSet.Spec.RunnerContainerSpec.Env, v1.EnvVar{
			// run.sh is PID 1 here and relays nothing to Runner.Listener unless this is set: https://github.com/actions/runner/blob/v2.335.1/src/Misc/layoutroot/run.sh
			Name:  "RUNNER_MANUALLY_TRAP_SIG",
			Value: "1",
		}),
		Resources:                scaleSet.Spec.RunnerContainerSpec.Resources,
		VolumeMounts:             scaleSet.Spec.RunnerContainerSpec.VolumeMounts,
		TerminationMessagePath:   v1.TerminationMessagePathDefault,
		TerminationMessagePolicy: v1.TerminationMessageReadFile,
	}
}

func (r *ScaleSetReconciler) buildRunnerVolumes(scaleSet *garV1.ScaleSet, volumeMounts []v1.VolumeMount) []v1.Volume {
	return filterMountedVolumes(scaleSet.Spec.Template.Spec.Volumes, volumeMounts)
}

func (r *ScaleSetReconciler) githubConfigURL(scaleSet *garV1.ScaleSet) string {
	if scaleSet.Spec.Repo == "" {
		return fmt.Sprintf("https://github.com/%s", scaleSet.Spec.Owner)
	}
	return fmt.Sprintf("https://github.com/%s/%s", scaleSet.Spec.Owner, scaleSet.Spec.Repo)
}

func (r *ScaleSetReconciler) newClient(ctx context.Context, scaleSet *garV1.ScaleSet) (*scaleset.Client, error) {
	systemInfo := scaleset.SystemInfo{
		System:    "github-actions-runner-controller",
		Version:   r.BinaryVersion,
		Subsystem: "listener",
	}

	if scaleSet.Spec.TokenSecretKeyRef != nil {
		var tokenSecret v1.Secret
		if err := r.Client.Get(
			ctx,
			client.ObjectKey{
				Name:      scaleSet.Spec.TokenSecretKeyRef.Name,
				Namespace: scaleSet.Namespace,
			},
			&tokenSecret,
		); err != nil {
			return nil, err
		}
		return scaleset.NewClientWithPersonalAccessToken(scaleset.NewClientWithPersonalAccessTokenConfig{
			GitHubConfigURL:     r.githubConfigURL(scaleSet),
			PersonalAccessToken: string(tokenSecret.Data[scaleSet.Spec.TokenSecretKeyRef.Key]),
			SystemInfo:          systemInfo,
		})
	}

	installationId, err := strconv.ParseInt(r.GitHubAppInstallationId, 10, 64)
	if err != nil {
		return nil, xerrors.Errorf("failed to parse github app installation id: %w", err)
	}

	return scaleset.NewClientWithGitHubApp(scaleset.ClientWithGitHubAppConfig{
		GitHubConfigURL: r.githubConfigURL(scaleSet),
		GitHubAppAuth: scaleset.GitHubAppAuth{
			ClientID:       r.GitHubAppClientId,
			InstallationID: installationId,
			PrivateKey:     r.GitHubAppPrivateKey,
		},
		SystemInfo: systemInfo,
	})
}

func (r *ScaleSetReconciler) scaleSetLabels(scaleSet *garV1.ScaleSet) []scaleset.Label {
	labels := []scaleset.Label{
		{Name: selfHostedLabel, Type: scaleSetLabelType},
		{Name: runnerLabel, Type: scaleSetLabelType},
	}
	for _, name := range scaleSet.Spec.Labels {
		if name == selfHostedLabel || name == runnerLabel {
			continue
		}
		labels = append(labels, scaleset.Label{Name: name, Type: scaleSetLabelType})
	}
	return labels
}

func labelNames(labels []scaleset.Label) []string {
	names := make([]string, 0, len(labels))
	for _, label := range labels {
		names = append(names, label.Name)
	}
	slices.Sort(names)
	return slices.Compact(names)
}

func (r *ScaleSetReconciler) registerScaleSet(ctx context.Context, actionsClient *scaleset.Client, scaleSet *garV1.ScaleSet) (int, error) {
	runnerGroupID := 1
	if scaleSet.Spec.RunnerGroup != "" {
		runnerGroup, err := actionsClient.GetRunnerGroupByName(ctx, scaleSet.Spec.RunnerGroup)
		if err != nil {
			return 0, xerrors.Errorf("failed to get runner group: %w", err)
		}
		runnerGroupID = runnerGroup.ID
	}

	registered, err := actionsClient.GetRunnerScaleSet(ctx, runnerGroupID, scaleSet.Name)
	if err != nil {
		return 0, xerrors.Errorf("failed to get runner scale set: %w", err)
	}
	if registered != nil {
		labels := r.scaleSetLabels(scaleSet)
		if !slices.Equal(labelNames(registered.Labels), labelNames(labels)) {
			if _, err := actionsClient.UpdateRunnerScaleSet(ctx, registered.ID, &scaleset.RunnerScaleSet{
				Name:          scaleSet.Name,
				RunnerGroupID: runnerGroupID,
				Labels:        labels,
				RunnerSetting: scaleset.RunnerSetting{
					DisableUpdate: true,
				},
			}); err != nil {
				return 0, xerrors.Errorf("failed to update runner scale set: %w", err)
			}
		}
		return registered.ID, nil
	}

	created, err := actionsClient.CreateRunnerScaleSet(ctx, &scaleset.RunnerScaleSet{
		Name:          scaleSet.Name,
		RunnerGroupID: runnerGroupID,
		Labels:        r.scaleSetLabels(scaleSet),
		RunnerSetting: scaleset.RunnerSetting{
			DisableUpdate: true,
		},
	})
	if err != nil {
		return 0, xerrors.Errorf("failed to create runner scale set: %w", err)
	}

	return created.ID, nil
}

func (r *ScaleSetReconciler) finalize(ctx context.Context, scaleSet *garV1.ScaleSet) error {
	if !controllerutil.ContainsFinalizer(scaleSet, scaleSetFinalizer) {
		return nil
	}

	actionsClient, scaleSetID, running := r.Listener.Session(scaleSet.UID)
	if !running && scaleSet.Status.ScaleSetID != 0 {
		// The listener is gone after a restart, so the client is rebuilt to unregister the scale set GitHub still holds.
		built, err := r.newClient(ctx, scaleSet)
		if err != nil {
			return err
		}
		actionsClient = built
		scaleSetID = scaleSet.Status.ScaleSetID
	}

	// Stopped first so a failing unregistration cannot leave the poll loop creating runners for a scale set being deleted.
	r.Listener.Stop(scaleSet.UID)

	if actionsClient != nil {
		if err := actionsClient.DeleteRunnerScaleSet(ctx, scaleSetID); err != nil {
			return xerrors.Errorf("failed to delete runner scale set: %w", err)
		}
	}

	runnable.DeleteSeries(scaleSet.Namespace, scaleSet.Name)

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		latest := &garV1.ScaleSet{}
		if err := r.Get(ctx, client.ObjectKeyFromObject(scaleSet), latest); err != nil {
			return client.IgnoreNotFound(err)
		}
		if !controllerutil.RemoveFinalizer(latest, scaleSetFinalizer) {
			return nil
		}
		return r.Update(ctx, latest)
	})
}

func (r *ScaleSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.APIReader = mgr.GetAPIReader()

	return ctrl.NewControllerManagedBy(mgr).
		For(&garV1.ScaleSet{}).
		Owns(&v1.ConfigMap{}).
		Owns(&v1.Secret{}).
		Owns(&batchV1.Job{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		Complete(r)
}
