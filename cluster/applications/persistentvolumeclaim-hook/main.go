package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"net/http"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func main() {
	var host string
	var port int
	var certDir string
	var metricsAddr string
	var enableHTTP2 bool
	var secureMetrics bool
	var probeAddr string
	flag.StringVar(&host, "host", "", "")
	flag.IntVar(&port, "port", 9443, "")
	flag.StringVar(&certDir, "certDir", "/var/k8s-webhook-server/serving-certs", "CertDir is the directory that contains the server key and certificate. The server key and certificate.")
	flag.StringVar(&metricsAddr, "metrics-bind-address", "0.0.0.0:8080", "The address the metric endpoint binds to.")
	flag.BoolVar(&secureMetrics, "metrics-secure", false, "If set the metrics endpoint is served securely")
	flag.BoolVar(&enableHTTP2, "enable-http2", false, "If set, HTTP/2 will be enabled for the metrics and webhook servers")
	flag.StringVar(&probeAddr, "health-probe-bind-address", "0.0.0.0:8081", "The address the probe endpoint binds to.")
	opts := zap.Options{}
	opts.BindFlags(flag.CommandLine)
	klog.InitFlags(flag.CommandLine)
	flag.Parse()

	zapLogger := zap.New(zap.UseFlagOptions(&opts))
	klog.SetLogger(zapLogger)
	ctrl.SetLogger(zapLogger)

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
	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	m, err := ctrl.NewManager(config.GetConfigOrDie(), ctrl.Options{
		Metrics: metricsserver.Options{
			BindAddress:   metricsAddr,
			SecureServing: secureMetrics,
			TLSOpts:       tlsOpts,
		},
		HealthProbeBindAddress: probeAddr,
		WebhookServer: webhook.NewServer(webhook.Options{
			Host:    host,
			Port:    port,
			CertDir: certDir,
		}),
	})
	if err != nil {
		entrypointLogger.Error(err, "unable to create manager")
		os.Exit(1)
	}

	webhookServer := m.GetWebhookServer()
	webhookServer.Register("/mutate", &webhook.Admission{Handler: &handler{client: m.GetClient(), decoder: admission.NewDecoder(m.GetScheme())}})

	if err := m.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		entrypointLogger.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := m.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		entrypointLogger.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	entrypointLogger.Info("starting manager")
	if err := m.Start(ctrl.SetupSignalHandler()); err != nil {
		entrypointLogger.Error(err, "unable to run manager")
		os.Exit(1)
	}
}

type handler struct {
	client  client.Client
	decoder *admission.Decoder
}

func (h *handler) Handle(ctx context.Context, req admission.Request) admission.Response {
	handlerLogger := ctrl.Log.WithName("handler")

	switch req.Kind {
	case metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "PersistentVolumeClaim"}:
		persistentVolumeClaim := &corev1.PersistentVolumeClaim{}
		if err := h.decoder.Decode(req, persistentVolumeClaim); err != nil {
			handlerLogger.Error(err, "unable to decode request")
			return admission.Errored(http.StatusBadRequest, err)
		}

		deployedPersistentVolumeClaim := &corev1.PersistentVolumeClaim{}
		if err := h.client.Get(ctx, client.ObjectKey{Name: persistentVolumeClaim.Name, Namespace: persistentVolumeClaim.Namespace}, deployedPersistentVolumeClaim); err != nil {
			handlerLogger.Error(err, "unable to get persistent volume claim")
			return admission.Errored(http.StatusInternalServerError, err)
		}

		patched := false
		if deployedPersistentVolumeClaim.Spec.Resources.Requests.Storage().Value() > persistentVolumeClaim.Spec.Resources.Requests.Storage().Value() {
			persistentVolumeClaim.Spec.Resources.Requests.Storage().Set(deployedPersistentVolumeClaim.Spec.Resources.Requests.Storage().Value())
			patched = true
		}

		if patched {
			marshalledPersistentVolumeClaim, err := json.Marshal(persistentVolumeClaim)
			if err != nil {
				handlerLogger.Error(err, "unable to marshal storage class")
				return admission.Errored(http.StatusInternalServerError, err)
			}
			return admission.PatchResponseFromRaw(req.Object.Raw, marshalledPersistentVolumeClaim)
		}
	case metav1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "StatefulSet"}:
		statefulSet := &appsv1.StatefulSet{}
		if err := h.decoder.Decode(req, statefulSet); err != nil {
			handlerLogger.Error(err, "unable to decode request")
			return admission.Errored(http.StatusBadRequest, err)
		}

		if len(statefulSet.Spec.VolumeClaimTemplates) == 0 {
			return admission.Allowed("")
		}

		deployedStatefulSet := &appsv1.StatefulSet{}
		if err := h.client.Get(ctx, client.ObjectKey{Name: statefulSet.Name, Namespace: statefulSet.Namespace}, deployedStatefulSet); err != nil {
			handlerLogger.Error(err, "unable to get stateful set")
			return admission.Errored(http.StatusInternalServerError, err)
		}

		m := make(map[string]*resource.Quantity)
		for _, deployedVolumeClaimTemplate := range deployedStatefulSet.Spec.VolumeClaimTemplates {
			m[deployedVolumeClaimTemplate.Name] = deployedVolumeClaimTemplate.Spec.Resources.Requests.Storage()
		}

		if len(m) == 0 {
			return admission.Allowed("")
		}

		patched := false
		for _, volumeClaimTemplate := range statefulSet.Spec.VolumeClaimTemplates {
			deployedStorage, ok := m[volumeClaimTemplate.Name]
			if ok && deployedStorage.Value() > volumeClaimTemplate.Spec.Resources.Requests.Storage().Value() {
				volumeClaimTemplate.Spec.Resources.Requests.Storage().Set(deployedStorage.Value())
				patched = true
			}
		}

		if patched {
			marshalledStatefulSet, err := json.Marshal(statefulSet)
			if err != nil {
				handlerLogger.Error(err, "unable to marshal stateful set")
				return admission.Errored(http.StatusInternalServerError, err)
			}
			return admission.PatchResponseFromRaw(req.Object.Raw, marshalledStatefulSet)
		}
	}

	return admission.Allowed("")
}
