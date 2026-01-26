package webhook

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"golang.org/x/xerrors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func Run(a *Args) error {
	if err := validator.New().Struct(a); err != nil {
		var errs validator.ValidationErrors
		errors.As(err, &errs)
		var messages []string
		for _, e := range errs {
			if e.ActualTag() == "oneof" {
				messages = append(messages, fmt.Sprintf("%s must be one of these [%s]", e.Field(), e.Param()))
			}
		}
		if len(messages) > 0 {
			err = xerrors.Errorf("%s: %w", strings.Join(messages, ", "), err)
		}
		return xerrors.Errorf("validation error: %w", err)
	}

	entrypointLogger := ctrl.Log.WithName("entrypoint")

	// if the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities. More specifically, disabling http/2 will
	// prevent from being vulnerable to the HTTP/2 Stream Cancelation and
	// Rapid Reset CVEs. For more information see:
	// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
	// - https://github.com/advisories/GHSA-4374-p667-p6c8
	disableHTTP2 := func(c *tls.Config) {
		entrypointLogger.Info("disabling http/2")
		c.NextProtos = []string{"http/1.1"}
	}

	tlsOpts := []func(*tls.Config){}
	if !a.EnableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	m, err := ctrl.NewManager(config.GetConfigOrDie(), ctrl.Options{
		Metrics: metricsserver.Options{
			BindAddress:   a.MetricsAddr,
			SecureServing: a.SecureMetrics,
			TLSOpts:       tlsOpts,
		},
		HealthProbeBindAddress: a.ProbeAddr,
		WebhookServer: webhook.NewServer(webhook.Options{
			Host:    a.Host,
			Port:    a.Port,
			CertDir: a.CertDir,
		}),
	})
	if err != nil {
		return xerrors.Errorf("unable to create manager: %w", err)
	}

	webhookServer := m.GetWebhookServer()
	webhookServer.Register("/mutate", &webhook.Admission{Handler: &handler{
		client:                 m.GetClient(),
		decoder:                admission.NewDecoder(m.GetScheme()),
		sidecarImage:           a.SidecarImage,
		terminationGracePeriod: a.TerminationGracePeriod,
	}})

	if err := m.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return xerrors.Errorf("unable to set up health check: %w", err)
	}
	if err := m.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		return xerrors.Errorf("unable to set up ready check: %w", err)
	}

	entrypointLogger.Info("starting manager")
	if err := m.Start(ctrl.SetupSignalHandler()); err != nil {
		return xerrors.Errorf("unable to run manager: %w", err)
	}

	return nil
}

type handler struct {
	client                 client.Client
	decoder                *admission.Decoder
	sidecarImage           string
	terminationGracePeriod time.Duration
}

func (h *handler) Handle(ctx context.Context, req admission.Request) admission.Response {
	handlerLogger := ctrl.Log.WithName("handler")

	switch req.Kind {
	case metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}:
		pod := &corev1.Pod{}
		if err := h.decoder.DecodeRaw(req.Object, pod); err != nil {
			handlerLogger.Error(err, "unable to decode object")
			return admission.Errored(http.StatusBadRequest, err)
		}

		if pod.Annotations["prometheus.io/wait"] != "true" {
			return admission.Allowed("")
		}

		metricsPort := pod.Annotations["prometheus.io/port"]
		if metricsPort == "" {
			return admission.Denied("prometheus.io/port annotation is required")
		}

		if pod.Annotations == nil {
			pod.Annotations = make(map[string]string)
		}
		pod.Annotations["prometheus.io/port"] = "65532"

		currentTerminationGracePeriodSeconds := int64(30)
		if pod.Spec.TerminationGracePeriodSeconds != nil {
			currentTerminationGracePeriodSeconds = *pod.Spec.TerminationGracePeriodSeconds
		}
		terminationGracePeriodSeconds := max(currentTerminationGracePeriodSeconds, int64(h.terminationGracePeriod.Seconds()))
		pod.Spec.TerminationGracePeriodSeconds = &terminationGracePeriodSeconds

		sidecarArgs := []string{
			"sidecar",
			fmt.Sprintf("http://127.0.0.1:%s", metricsPort),
			"--address", fmt.Sprintf("0.0.0.0:%d", 65532),
			"--lameduck", "1s",
			"--termination-grace-period", fmt.Sprintf("%ds", terminationGracePeriodSeconds),
		}

		sidecarSecurityContext := &corev1.SecurityContext{
			AllowPrivilegeEscalation: ptr.To(false),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{"ALL"},
			},
			ReadOnlyRootFilesystem: ptr.To(true),
			RunAsUser:              ptr.To[int64](65532),
			RunAsNonRoot:           ptr.To(true),
			SeccompProfile: &corev1.SeccompProfile{
				Type: corev1.SeccompProfileTypeRuntimeDefault,
			},
		}

		sidecarContainer := corev1.Container{
			Name:            "prometheus-metrics-proxy",
			SecurityContext: sidecarSecurityContext,
			Image:           h.sidecarImage,
			ImagePullPolicy: corev1.PullAlways,
			Args:            sidecarArgs,
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("10m"),
					corev1.ResourceMemory: resource.MustParse("16Mi"),
				},
			},
		}

		pod.Spec.Containers = append(pod.Spec.Containers, sidecarContainer)

		marshalledPod, err := json.Marshal(pod)
		if err != nil {
			handlerLogger.Error(err, "unable to marshal pod")
			return admission.Errored(http.StatusInternalServerError, err)
		}
		return admission.PatchResponseFromRaw(req.Object.Raw, marshalledPod)
	}

	return admission.Allowed("")
}
