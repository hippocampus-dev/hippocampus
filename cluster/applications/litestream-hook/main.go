package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	admissionV1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
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
	defaultGroup := "litestream-hook.kaidotio.github.io"
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
	var enableSidecarContainers bool
	flag.StringVar(&host, "host", envOrDefaultValue("HOST", ""), "")
	flag.IntVar(&port, "port", envOrDefaultValue("PORT", 9443), "")
	flag.StringVar(&certDir, "certDir", envOrDefaultValue("CERT_DIR", "/var/k8s-webhook-server/serving-certs"), "CertDir is the directory that contains the server key and certificate. The server key and certificate.")
	flag.StringVar(&metricsAddr, "metrics-bind-address", envOrDefaultValue("METRICS_BIND_ADDRESS", "0.0.0.0:8080"), "The address the metric endpoint binds to.")
	flag.BoolVar(&secureMetrics, "metrics-secure", envOrDefaultValue("METRICS_SECURE", false), "If set the metrics endpoint is served securely")
	flag.BoolVar(&enableHTTP2, "enable-http2", envOrDefaultValue("ENABLE_HTTP2", false), "If set, HTTP/2 will be enabled for the metrics and webhook servers")
	flag.StringVar(&probeAddr, "health-probe-bind-address", envOrDefaultValue("HEALTH_PROBE_BIND_ADDRESS", "0.0.0.0:8081"), "The address the probe endpoint binds to.")
	flag.BoolVar(&enableSidecarContainers, "enable-sidecar-containers", envOrDefaultValue("ENABLE_SIDECAR_CONTAINERS", false), "Enable native sidecar containers (requires Kubernetes 1.28+)")
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
	webhookServer.Register("/mutate", &webhook.Admission{Handler: &handler{
		client:                  m.GetClient(),
		decoder:                 admission.NewDecoder(m.GetScheme()),
		enableSidecarContainers: enableSidecarContainers,
	}})

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
	client                  client.Client
	decoder                 *admission.Decoder
	enableSidecarContainers bool
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

		if pod.Annotations[fmt.Sprintf("%s/inject", apiGroup())] != "true" {
			return admission.Allowed("")
		}

		storageAnnotation := func() string {
			storageAnnotation := pod.Annotations[fmt.Sprintf("%s/storage", apiGroup())]
			if storageAnnotation == "" {
				return "1Gi"
			}
			return storageAnnotation
		}()
		imageAnnotation := func() string {
			imageAnnotation := pod.Annotations[fmt.Sprintf("%s/image", apiGroup())]
			if imageAnnotation == "" {
				return "litestream/litestream:0.3"
			}
			return imageAnnotation
		}()
		bucketAnnotation := func() string {
			bucketAnnotation := pod.Annotations[fmt.Sprintf("%s/bucket", apiGroup())]
			if bucketAnnotation == "" {
				return "litestream"
			}
			return bucketAnnotation
		}()
		secretAnnotation := func() string {
			secretAnnotation := pod.Annotations[fmt.Sprintf("%s/secret", apiGroup())]
			if secretAnnotation == "" {
				return "litestream-hook-secret"
			}
			return secretAnnotation
		}()
		pathAnnotation := func() []string {
			pathAnnotation := pod.Annotations[fmt.Sprintf("%s/path", apiGroup())]
			if pathAnnotation == "" {
				return []string{"/mnt/litestream.db"}
			}

			var paths []string
			for _, path := range strings.Split(pathAnnotation, ",") {
				path = strings.TrimSpace(path)
				if path != "" {
					paths = append(paths, path)
				}
			}

			if len(paths) == 0 {
				return []string{"/mnt/litestream.db"}
			}

			return paths
		}()

		storage := resource.MustParse(storageAnnotation)

		pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
			Name: "litestream-hook-storage",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					Medium:    corev1.StorageMediumDefault,
					SizeLimit: &storage,
				},
			},
		})

		pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
			Name: "litestream-hook-config",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					Medium: corev1.StorageMediumMemory,
				},
			},
		})

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

		generateLitestreamConfiguration := func() string {
			configuration := `
cat <<EOS > /etc/litestream/litestream.yml
dbs:`

			endpointAnnotation := pod.Annotations[fmt.Sprintf("%s/endpoint", apiGroup())]

			for _, path := range pathAnnotation {
				if endpointAnnotation != "" {
					configuration += fmt.Sprintf(`
  - path: %s
    replicas:
      - type: s3
        bucket: %s
        endpoint: %s
        force-path-style: true`, path, bucketAnnotation, endpointAnnotation)
				} else {
					configuration += fmt.Sprintf(`
  - path: %s
    replicas:
      - type: s3
        bucket: %s`, path, bucketAnnotation)
				}
			}

			configuration += "\nEOS"
			return configuration
		}()

		pod.Spec.InitContainers = append(pod.Spec.InitContainers, corev1.Container{
			Name:  "litestream-hook-init",
			Image: "busybox",
			Command: []string{
				"sh",
				"-c",
			},
			Args: []string{
				generateLitestreamConfiguration,
			},
			SecurityContext: sidecarSecurityContext,
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "litestream-hook-config",
					MountPath: "/etc/litestream",
				},
			},
		})

		for i, path := range pathAnnotation {
			pod.Spec.InitContainers = append(pod.Spec.InitContainers, corev1.Container{
				Name:  fmt.Sprintf("litestream-hook-restore-%d", i),
				Image: imageAnnotation,
				Args: []string{
					"restore",
					"-config",
					"/etc/litestream/litestream.yml",
					"-if-db-not-exists",
					"-if-replica-exists",
					path,
				},
				Env: []corev1.EnvVar{
					{
						Name: "LITESTREAM_ACCESS_KEY_ID",
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: secretAnnotation,
								},
								Key: "AWS_ACCESS_KEY_ID",
							},
						},
					},
					{
						Name: "LITESTREAM_SECRET_ACCESS_KEY",
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: secretAnnotation,
								},
								Key: "AWS_SECRET_ACCESS_KEY",
							},
						},
					},
				},
				SecurityContext: sidecarSecurityContext,
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      "litestream-hook-config",
						MountPath: "/etc/litestream",
					},
					{
						Name:      "litestream-hook-storage",
						MountPath: filepath.Dir(path),
					},
				},
			})
		}

		for i, container := range pod.Spec.Containers {
			mountedDirectories := make(map[string]bool)

			for _, path := range pathAnnotation {
				directory := filepath.Dir(path)
				if !mountedDirectories[directory] {
					pod.Spec.Containers[i].VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
						Name:      "litestream-hook-storage",
						MountPath: directory,
					})
					mountedDirectories[directory] = true
				}
			}
		}

		replicateContainer := corev1.Container{
			Name:  "litestream-hook-replicate",
			Image: imageAnnotation,
			Args: []string{
				"replicate",
				"-config",
				"/etc/litestream/litestream.yml",
			},
			Env: []corev1.EnvVar{
				{
					Name: "LITESTREAM_ACCESS_KEY_ID",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: secretAnnotation,
							},
							Key: "AWS_ACCESS_KEY_ID",
						},
					},
				},
				{
					Name: "LITESTREAM_SECRET_ACCESS_KEY",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: secretAnnotation,
							},
							Key: "AWS_SECRET_ACCESS_KEY",
						},
					},
				},
			},
			SecurityContext: sidecarSecurityContext,
			Resources: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceEphemeralStorage: storage,
				},
			},
			VolumeMounts: func() []corev1.VolumeMount {
				mounts := []corev1.VolumeMount{
					{
						Name:      "litestream-hook-config",
						MountPath: "/etc/litestream",
					},
				}

				mountedDirectories := make(map[string]bool)

				for _, path := range pathAnnotation {
					directory := filepath.Dir(path)
					if !mountedDirectories[directory] {
						mounts = append(mounts, corev1.VolumeMount{
							Name:      "litestream-hook-storage",
							MountPath: directory,
						})
						mountedDirectories[directory] = true
					}
				}

				return mounts
			}(),
		}

		if h.enableSidecarContainers {
			replicateContainer.RestartPolicy = func(p corev1.ContainerRestartPolicy) *corev1.ContainerRestartPolicy { return &p }(corev1.ContainerRestartPolicyAlways)
			pod.Spec.InitContainers = append(pod.Spec.InitContainers, replicateContainer)
		} else {
			pod.Spec.Containers = append(pod.Spec.Containers, replicateContainer)
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
