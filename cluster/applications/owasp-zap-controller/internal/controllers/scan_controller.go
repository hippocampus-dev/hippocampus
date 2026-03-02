package controllers

import (
	"context"
	"fmt"
	zapV1 "owasp-zap-controller/api/v1"
	"strings"

	"github.com/go-logr/logr"
	"golang.org/x/xerrors"
	"gopkg.in/yaml.v3"
	batchV1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type ScanReconciler struct {
	client.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder

	CallbackHost string
}

func (r *ScanReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	scan := &zapV1.Scan{}
	if err := r.Get(ctx, req.NamespacedName, scan); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if scan.Status.ObservedGeneration >= scan.Generation {
		return ctrl.Result{}, nil
	}

	scan.Status.ObservedGeneration = scan.Generation
	if err := r.Status().Update(ctx, scan); err != nil {
		return ctrl.Result{}, err
	}

	automationConfigMapName := fmt.Sprintf("%s-automation-%d", scan.Name, scan.Generation)
	if err := r.createAutomationConfigMap(ctx, scan, automationConfigMapName); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.createJob(ctx, scan, automationConfigMapName); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *ScanReconciler) createAutomationConfigMap(ctx context.Context, scan *zapV1.Scan, configMapName string) error {
	automationCopy := scan.Spec.Automation.DeepCopy()

	automationCopy.InjectCredentialPlaceholders()

	automationYAMLBytes, err := yaml.Marshal(automationCopy)
	if err != nil {
		return xerrors.Errorf("failed to marshal automation yaml: %w", err)
	}

	automationConfigMap := &corev1.ConfigMap{
		ObjectMeta: metaV1.ObjectMeta{
			Name:      configMapName,
			Namespace: scan.Namespace,
			Labels: map[string]string{
				"app":        "zap-scanner",
				"scan":       scan.Name,
				"controller": "scan-controller",
			},
		},
		Data: map[string]string{
			"automation.yaml": string(automationYAMLBytes),
		},
	}

	if err := controllerutil.SetControllerReference(scan, automationConfigMap, r.Scheme); err != nil {
		return xerrors.Errorf("failed to set controller reference: %w", err)
	}

	if err := r.Create(ctx, automationConfigMap); err != nil {
		if apierrors.IsAlreadyExists(err) {
			r.Log.Info("ConfigMap already exists", "configmap", configMapName)
			return nil
		}
		return xerrors.Errorf("failed to create configmap: %w", err)
	}

	r.Recorder.Eventf(scan, corev1.EventTypeNormal, "ConfigMapCreated", "Created configmap %s for scan", configMapName)
	return nil
}

func (r *ScanReconciler) createJob(ctx context.Context, scan *zapV1.Scan, automationConfigMapName string) error {
	jobName := fmt.Sprintf("%s-zap-scan-%d", scan.Name, scan.Generation)
	automationFile := "/etc/zap/automation.yaml"

	job := &batchV1.Job{
		ObjectMeta: metaV1.ObjectMeta{
			Name:      jobName,
			Namespace: scan.Namespace,
		},
		Spec: batchV1.JobSpec{
			BackoffLimit: ptr.To[int32](3),
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyOnFailure,
					Containers: []corev1.Container{
						{
							Name: "zap",
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: ptr.To(false),
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
								},
								RunAsUser:    ptr.To[int64](1000),
								RunAsNonRoot: ptr.To(true),
								SeccompProfile: &corev1.SeccompProfile{
									Type: corev1.SeccompProfileTypeRuntimeDefault,
								},
							},
							Image:           scan.Spec.ZapImage,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command:         []string{"sh", "-c"},
							Args:            []string{r.buildScanScript(automationFile, scan.Spec.Automation.ReportPaths(), scan.Name, scan.Namespace)},
							Env:             scan.Spec.Automation.BuildEnvVars(),
							Resources:       scan.Spec.Resources,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "tmp",
									MountPath: "/tmp",
								},
								{
									Name:      "automation-config",
									MountPath: "/etc/zap",
									ReadOnly:  true,
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "tmp",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{
									Medium: corev1.StorageMediumMemory,
								},
							},
						},
						{
							Name: "automation-config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: automationConfigMapName,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	secretNames := scan.Spec.Automation.CollectSecretNames()
	for i, secretName := range secretNames {
		volumeName := fmt.Sprintf("secret-%d", i)
		job.Spec.Template.Spec.Volumes = append(job.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secretName,
				},
			},
		})
		job.Spec.Template.Spec.Containers[0].VolumeMounts = append(
			job.Spec.Template.Spec.Containers[0].VolumeMounts,
			corev1.VolumeMount{
				Name:      volumeName,
				MountPath: fmt.Sprintf("/mnt/secrets/%s", secretName),
				ReadOnly:  true,
			},
		)
	}

	if err := controllerutil.SetControllerReference(scan, job, r.Scheme); err != nil {
		return xerrors.Errorf("failed to set controller reference: %w", err)
	}

	if err := r.Create(ctx, job); err != nil {
		return xerrors.Errorf("failed to create job: %w", err)
	}

	r.Recorder.Eventf(scan, corev1.EventTypeNormal, "JobCreated", "Created job %s for scan", jobName)
	return nil
}

func (r *ScanReconciler) buildScanScript(automationFile string, reportPaths []string, scanName string, namespace string) string {
	script := fmt.Sprintf(`
set -e

zap.sh -cmd -autorun '%s'
ZAP_EXIT_CODE=$?

PAYLOAD_FILE=$(mktemp)
echo '{}' > $PAYLOAD_FILE

`, strings.ReplaceAll(automationFile, "'", "'\\''"))

	for _, reportPath := range reportPaths {
		escapedPath := strings.ReplaceAll(reportPath, "'", "'\\''")

		script += fmt.Sprintf(`
if [ -f "%s" ]; then
  t=$(mktemp)
  cat "%s" | jq -Rs . > $t
  jq --arg path "%s" --slurpfile content $t '.[$path] = $content[0]' $PAYLOAD_FILE > $PAYLOAD_FILE.tmp
  mv $PAYLOAD_FILE.tmp $PAYLOAD_FILE
  rm -f $t
fi
`, escapedPath, escapedPath, reportPath)
	}

	script += fmt.Sprintf(`
curl -sSL -X PATCH \
  -H "Content-Type: application/json" \
  -d @$PAYLOAD_FILE \
  "http://%s/api/%s/%s/%s/%s/%s/reports"

rm -f $PAYLOAD_FILE

exit $ZAP_EXIT_CODE
`, r.CallbackHost, namespace, zapV1.GroupVersion.Group, zapV1.GroupVersion.Version, "scan", scanName)
	return script
}

func (r *ScanReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&zapV1.Scan{}).
		Owns(&batchV1.Job{}).
		Owns(&corev1.ConfigMap{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		Complete(r)
}
