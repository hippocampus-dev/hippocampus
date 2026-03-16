package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	admissionV1 "k8s.io/api/admission/v1"
	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
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

func apiGroup() string {
	defaultGroup := "lifecycle-job-hook.kaidotio.github.io"
	if v, ok := os.LookupEnv("VARIANT"); ok {
		return fmt.Sprintf("%s.%s", v, defaultGroup)
	}
	return defaultGroup
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
	webhookServer.Register("/validate", &webhook.Admission{Handler: &handler{client: m.GetClient(), decoder: admission.NewDecoder(m.GetScheme())}})

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
		operation admissionV1.Operation
	}
	switch (pair{req.Kind, req.Operation}) {
	case pair{metav1.GroupVersionKind{Group: "batch", Version: "v1", Kind: "Job"}, admissionV1.Update}:
		job := &batchv1.Job{}
		if err := h.decoder.DecodeRaw(req.Object, job); err != nil {
			handlerLogger.Error(err, "unable to decode object")
			return admission.Errored(http.StatusBadRequest, err)
		}

		switch job.Annotations[fmt.Sprintf("%s/hook", apiGroup())] {
		case "":
			return admission.Allowed("")
		case "PostComplete":
			oldJob := &batchv1.Job{}
			if err := h.decoder.DecodeRaw(req.OldObject, oldJob); err != nil {
				handlerLogger.Error(err, "unable to decode old object")
				return admission.Errored(http.StatusBadRequest, err)
			}

			if oldJob.Status.CompletionTime == nil && job.Status.CompletionTime != nil {
				newJob, err := h.createNewJobFromMetadata(ctx, job.TypeMeta, job.ObjectMeta)
				if err != nil {
					if apierrors.IsNotFound(err) {
						return admission.Denied("job template is not found")
					}
					handlerLogger.Error(err, "unable to create new job")
					return admission.Errored(http.StatusInternalServerError, err)
				}

				if newJob == nil {
					return admission.Denied("job template is not found")
				}

				if err := h.client.Create(ctx, newJob); err != nil {
					handlerLogger.Error(err, "unable to create object")
					return admission.Errored(http.StatusInternalServerError, err)
				}
			}
		default:
			return admission.Denied("hook annotation is invalid")
		}
	case pair{metav1.GroupVersionKind{Group: "batch", Version: "v1", Kind: "CronJob"}, admissionV1.Update}:
		cronJob := &batchv1.CronJob{}
		if err := h.decoder.DecodeRaw(req.Object, cronJob); err != nil {
			handlerLogger.Error(err, "unable to decode object")
			return admission.Errored(http.StatusBadRequest, err)
		}

		switch cronJob.Annotations[fmt.Sprintf("%s/hook", apiGroup())] {
		case "":
			return admission.Allowed("")
		case "PostComplete":
			oldCronJob := &batchv1.CronJob{}
			if err := h.decoder.DecodeRaw(req.OldObject, oldCronJob); err != nil {
				handlerLogger.Error(err, "unable to decode old object")
				return admission.Errored(http.StatusBadRequest, err)
			}

			if len(oldCronJob.Status.Active) != 0 && len(cronJob.Status.Active) == 0 {
				newJob, err := h.createNewJobFromMetadata(ctx, cronJob.TypeMeta, cronJob.ObjectMeta)
				if err != nil {
					if apierrors.IsNotFound(err) {
						return admission.Denied("job template is not found")
					}
					handlerLogger.Error(err, "unable to create new job")
					return admission.Errored(http.StatusInternalServerError, err)
				}

				if newJob == nil {
					return admission.Denied("job template is not found")
				}

				if err := h.client.Create(ctx, newJob); err != nil {
					handlerLogger.Error(err, "unable to create object")
					return admission.Errored(http.StatusInternalServerError, err)
				}
			}
		default:
			return admission.Denied("hook annotation is invalid")
		}
	}

	return admission.Allowed("")
}

func (h *handler) createNewJobFromMetadata(ctx context.Context, typeMeta metav1.TypeMeta, objectMeta metav1.ObjectMeta) (*batchv1.Job, error) {
	group, ok := objectMeta.Annotations[fmt.Sprintf("%s/job-template-apiGroup", apiGroup())]
	if !ok {
		return nil, nil
	}
	version, ok := objectMeta.Annotations[fmt.Sprintf("%s/job-template-version", apiGroup())]
	if !ok {
		return nil, nil
	}
	kind, ok := objectMeta.Annotations[fmt.Sprintf("%s/job-template-kind", apiGroup())]
	if !ok {
		return nil, nil
	}
	name, ok := objectMeta.Annotations[fmt.Sprintf("%s/job-template-name", apiGroup())]
	if !ok {
		return nil, nil
	}

	newJob := &batchv1.Job{}

	unix := time.Now().UnixNano()

	gvk := metav1.GroupVersionKind{Group: group, Version: version, Kind: kind}
	switch gvk {
	case metav1.GroupVersionKind{Group: "batch", Version: "v1", Kind: "Job"}:
		job := &batchv1.Job{}
		if err := h.client.Get(ctx, client.ObjectKey{Namespace: objectMeta.Namespace, Name: name}, job); err != nil {
			return nil, err
		}

		newJob.Name = fmt.Sprintf("%s-%s", job.Name, strconv.Itoa(int(unix)))
		newJob.Namespace = job.Namespace
		newJob.Spec = job.Spec

		// Set an owner reference to the caller
		newJob.SetOwnerReferences([]metav1.OwnerReference{
			{
				APIVersion:         typeMeta.APIVersion,
				Kind:               typeMeta.Kind,
				Name:               objectMeta.Name,
				UID:                objectMeta.UID,
				Controller:         ptr.To(true),
				BlockOwnerDeletion: ptr.To(true),
			},
		})
	case metav1.GroupVersionKind{Group: "batch", Version: "v1", Kind: "CronJob"}:
		cronJob := &batchv1.CronJob{}
		if err := h.client.Get(ctx, client.ObjectKey{Namespace: objectMeta.Namespace, Name: name}, cronJob); err != nil {
			return nil, err
		}

		newJob.Name = fmt.Sprintf("%s-%s", cronJob.Name, strconv.Itoa(int(unix)))
		newJob.Namespace = cronJob.Namespace
		newJob.Spec = cronJob.Spec.JobTemplate.Spec

		// Set an owner reference to the job template
		newJob.SetOwnerReferences([]metav1.OwnerReference{
			{
				APIVersion:         cronJob.APIVersion,
				Kind:               cronJob.Kind,
				Name:               cronJob.Name,
				UID:                cronJob.UID,
				Controller:         ptr.To(true),
				BlockOwnerDeletion: ptr.To(true),
			},
		})
	}

	newJob.Spec.Selector = nil
	if newJob.Spec.Template.ObjectMeta.Labels == nil {
		newJob.Spec.Template.ObjectMeta.Labels = map[string]string{}
	}
	newJob.Spec.Template.Labels["batch.kubernetes.io/job-name"] = newJob.Name
	newJob.Spec.Template.Labels["job-name"] = newJob.Name
	delete(newJob.Spec.Template.Labels, "batch.kubernetes.io/controller-uid")
	delete(newJob.Spec.Template.Labels, "controller-uid")

	return newJob, nil
}
