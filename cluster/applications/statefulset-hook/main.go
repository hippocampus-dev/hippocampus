package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"net/http"
	"os"
	"strconv"
	"time"

	admissionv1 "k8s.io/api/admission/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func envOrDefaultValue[T any](key string, defaultValue T) T {
	value, exists := os.LookupEnv(key)
	if !exists {
		return defaultValue
	}

	switch any(defaultValue).(type) {
	case string:
		return any(value).(T)
	case int:
		if intValue, err := strconv.Atoi(value); err == nil {
			return any(intValue).(T)
		}
	case int64:
		if intValue, err := strconv.ParseInt(value, 10, 64); err == nil {
			return any(intValue).(T)
		}
	case uint:
		if uintValue, err := strconv.ParseUint(value, 10, 0); err == nil {
			return any(uint(uintValue)).(T)
		}
	case uint64:
		if uintValue, err := strconv.ParseUint(value, 10, 64); err == nil {
			return any(uintValue).(T)
		}
	case float64:
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			return any(floatValue).(T)
		}
	case bool:
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return any(boolValue).(T)
		}
	case time.Duration:
		if durationValue, err := time.ParseDuration(value); err == nil {
			return any(durationValue).(T)
		}
	}

	return defaultValue
}

func main() {
	var host string
	var port int
	var certDir string
	var metricsAddr string
	var enableHTTP2 bool
	var secureMetrics bool
	var probeAddr string
	flag.StringVar(&host, "host", envOrDefaultValue("HOST", ""), "")
	flag.IntVar(&port, "port", envOrDefaultValue("PORT", 9443), "")
	flag.StringVar(&certDir, "certDir", envOrDefaultValue("CERT_DIR", "/var/k8s-webhook-server/serving-certs"), "CertDir is the directory that contains the server key and certificate. The server key and certificate.")
	flag.StringVar(&metricsAddr, "metrics-bind-address", envOrDefaultValue("METRICS_BIND_ADDRESS", "0.0.0.0:8080"), "The address the metric endpoint binds to.")
	flag.BoolVar(&secureMetrics, "metrics-secure", envOrDefaultValue("METRICS_SECURE", false), "If set the metrics endpoint is served securely")
	flag.BoolVar(&enableHTTP2, "enable-http2", envOrDefaultValue("ENABLE_HTTP2", false), "If set, HTTP/2 will be enabled for the metrics and webhook servers")
	flag.StringVar(&probeAddr, "health-probe-bind-address", envOrDefaultValue("HEALTH_PROBE_BIND_ADDRESS", "0.0.0.0:8081"), "The address the probe endpoint binds to.")
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

	type pair struct {
		gvk       metav1.GroupVersionKind
		operation admissionv1.Operation
	}
	switch (pair{req.Kind, req.Operation}) {
	case pair{metav1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "StatefulSet"}, admissionv1.Create}:
		statefulSet := &appsv1.StatefulSet{}
		if err := h.decoder.DecodeRaw(req.Object, statefulSet); err != nil {
			handlerLogger.Error(err, "unable to decode object")
			return admission.Errored(http.StatusBadRequest, err)
		}

		selector, err := metav1.LabelSelectorAsSelector(statefulSet.Spec.Selector)
		if err != nil {
			handlerLogger.Error(err, "unable to convert label selector")
			return admission.Errored(http.StatusInternalServerError, err)
		}

		podList := &corev1.PodList{}
		if err := h.client.List(ctx, podList,
			client.InNamespace(req.Namespace),
			client.MatchingLabelsSelector{Selector: selector}); err != nil {
			handlerLogger.Error(err, "unable to list pods")
			return admission.Errored(http.StatusInternalServerError, err)
		}

		count := int32(0)
		for _, pod := range podList.Items {
			if pod.DeletionTimestamp != nil {
				continue
			}
			if pod.Status.Phase == corev1.PodSucceeded {
				continue
			}
			count++
		}

		if count == 0 {
			handlerLogger.Info("no matching pods found, allowing original replicas", "namespace", req.Namespace, "name", statefulSet.Name)
			return admission.Allowed("")
		}

		handlerLogger.Info("setting replicas to existing pod count", "namespace", req.Namespace, "name", statefulSet.Name, "count", count)

		statefulSet.Spec.Replicas = &count

		marshalledStatefulSet, err := json.Marshal(statefulSet)
		if err != nil {
			handlerLogger.Error(err, "unable to marshal stateful set")
			return admission.Errored(http.StatusInternalServerError, err)
		}

		return admission.PatchResponseFromRaw(req.Object.Raw, marshalledStatefulSet)
	}

	return admission.Allowed("")
}
