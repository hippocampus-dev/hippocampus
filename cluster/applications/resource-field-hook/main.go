package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"text/template"
	"time"

	"golang.org/x/xerrors"
	admissionV1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
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

func apiGroup() string {
	defaultGroup := "resource-field-hook.kaidotio.github.io"
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
	flag.StringVar(&host, "host", envOrDefaultValue("HOST", ""), "Server host")
	flag.IntVar(&port, "port", envOrDefaultValue("PORT", 9443), "Server port")
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
	decoder admission.Decoder
}

func (h *handler) Handle(ctx context.Context, req admission.Request) admission.Response {
	handlerLogger := ctrl.Log.WithName("handler")

	type pair struct {
		gvk       metav1.GroupVersionKind
		operation admissionV1.Operation
	}
	switch (pair{req.Kind, req.Operation}) {
	case pair{metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}, admissionV1.Create}:
		pod := &corev1.Pod{}
		if err := h.decoder.DecodeRaw(req.Object, pod); err != nil {
			handlerLogger.Error(err, "unable to decode object")
			return admission.Errored(http.StatusBadRequest, err)
		}

		for _, containers := range [][]corev1.Container{pod.Spec.InitContainers, pod.Spec.Containers} {
			for i := range containers {
				container := &containers[i]
				for j := range container.Env {
					env := &container.Env[j]

					if env.ValueFrom == nil || env.ValueFrom.ResourceFieldRef == nil {
						continue
					}
					ref := env.ValueFrom.ResourceFieldRef

					expression, ok := pod.Annotations[fmt.Sprintf("%s/%s", apiGroup(), env.Name)]
					if !ok {
						continue
					}

					if !ref.Divisor.IsZero() && ref.Divisor.Cmp(oneQuantity) != 0 {
						handlerLogger.Error(
							xerrors.Errorf("container %q env %q: resourceFieldRef sets divisor %q, which is not reproduced here", container.Name, env.Name, ref.Divisor.String()),
							"unable to scale env var",
						)
						continue
					}

					if !strings.HasSuffix(ref.Resource, "memory") {
						handlerLogger.Error(
							xerrors.Errorf("container %q env %q: resource %q is not a memory resource", container.Name, env.Name, ref.Resource),
							"unable to scale env var",
						)
						continue
					}

					quantity, ok := func() (int64, bool) {
						target := container
						if ref.ContainerName != "" && ref.ContainerName != container.Name {
							target = nil
							for _, others := range [][]corev1.Container{pod.Spec.InitContainers, pod.Spec.Containers} {
								for k := range others {
									if others[k].Name == ref.ContainerName {
										target = &others[k]
									}
								}
							}
							if target == nil {
								return 0, false
							}
						}

						var list corev1.ResourceList
						switch {
						case strings.HasPrefix(ref.Resource, "limits."):
							list = target.Resources.Limits
						case strings.HasPrefix(ref.Resource, "requests."):
							list = target.Resources.Requests
						default:
							return 0, false
						}

						q, ok := list[corev1.ResourceName(strings.SplitN(ref.Resource, ".", 2)[1])]
						if !ok || q.IsZero() {
							return 0, false
						}
						return q.Value(), true
					}()
					if !ok {
						handlerLogger.Error(
							xerrors.Errorf("container %q env %q: resource %q resolves to nothing on this pod", container.Name, env.Name, ref.Resource),
							"unable to scale env var",
						)
						continue
					}

					scaled, err := render(expression, quantity)
					if err != nil {
						handlerLogger.Error(
							xerrors.Errorf("container %q env %q: failed to evaluate %q against %d: %w", container.Name, env.Name, expression, quantity, err),
							"unable to scale env var",
						)
						continue
					}

					env.Value = strconv.FormatInt(scaled, 10)
					env.ValueFrom = nil
				}
			}
		}

		marshalledPod, err := json.Marshal(pod)
		if err != nil {
			handlerLogger.Error(err, "unable to marshal pod")
			return admission.Errored(http.StatusInternalServerError, err)
		}
		return admission.PatchResponseFromRaw(req.Object.Raw, marshalledPod)
	}

	return admission.Allowed("")
}

var oneQuantity = resource.MustParse("1")

var templateFuncs = template.FuncMap{
	"quantity": func(s string) (int64, error) {
		q, err := resource.ParseQuantity(s)
		if err != nil {
			return 0, xerrors.Errorf("failed to parse quantity %q: %w", s, err)
		}
		return q.Value(), nil
	},
	"scale": func(v int64, f float64) (int64, error) {
		if f <= 0 || f > 1 {
			return 0, xerrors.Errorf("scale factor %v is not above 0 and at most 1", f)
		}
		return int64(math.Floor(float64(v) * f)), nil
	},
	"sub": func(a int64, b int64) int64 { return a - b },
	"add": func(a int64, b int64) int64 { return a + b },
	"min": func(a int64, b int64) int64 {
		if a < b {
			return a
		}
		return b
	},
	"max": func(a int64, b int64) int64 {
		if a > b {
			return a
		}
		return b
	},
}

type templateData struct {
	Value int64
}

func render(expression string, value int64) (int64, error) {
	tmpl, err := template.New("expression").Funcs(templateFuncs).Option("missingkey=error").Parse(expression)
	if err != nil {
		return 0, xerrors.Errorf("failed to parse expression: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, templateData{Value: value}); err != nil {
		return 0, xerrors.Errorf("failed to execute expression: %w", err)
	}

	rendered := strings.TrimSpace(buf.String())
	scaled, err := strconv.ParseInt(rendered, 10, 64)
	if err != nil {
		return 0, xerrors.Errorf("expression produced %q, which is not a byte count: %w", rendered, err)
	}
	if scaled <= 0 {
		return 0, xerrors.Errorf("expression produced %d, which leaves nothing to set", scaled)
	}
	return scaled, nil
}
