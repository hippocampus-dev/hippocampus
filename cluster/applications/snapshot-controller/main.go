package main

import (
	"context"
	"crypto/tls"
	"flag"
	"os"
	ssV1 "snapshot-controller/api/v1"
	"snapshot-controller/internal/capture"
	"snapshot-controller/internal/controllers"
	"snapshot-controller/internal/runnable"
	"snapshot-controller/internal/storage"
	"strconv"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	// +kubebuilder:scaffold:imports
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(ssV1.AddToScheme(scheme))
}

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
	var metricsAddr string
	var secureMetrics bool
	var enableHTTP2 bool
	var probeAddr string
	var enableLeaderElection bool

	var distributed bool
	var distributedCallbackHost string
	var distributedWorkerImage string

	flag.StringVar(&metricsAddr, "metrics-bind-address", envOrDefaultValue("METRICS_BIND_ADDRESS", "0.0.0.0:8080"), "The address the metric endpoint binds to.")
	flag.BoolVar(&secureMetrics, "metrics-secure", envOrDefaultValue("METRICS_SECURE", false), "If set the metrics endpoint is served securely")
	flag.BoolVar(&enableHTTP2, "enable-http2", envOrDefaultValue("ENABLE_HTTP2", false), "If set, HTTP/2 will be enabled for the metrics and webhook servers")
	flag.StringVar(&probeAddr, "health-probe-bind-address", envOrDefaultValue("HEALTH_PROBE_BIND_ADDRESS", "0.0.0.0:8081"), "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", envOrDefaultValue("ENABLE_LEADER_ELECTION", false),
		"Enable leader election for controller manager.")

	flag.BoolVar(&distributed, "distributed", envOrDefaultValue("DISTRIBUTED", false), "Enable distributed mode using Jobs/CronJobs")
	flag.StringVar(&distributedCallbackHost, "distributed-callback-host", envOrDefaultValue("DISTRIBUTED_CALLBACK_HOST", "snapshot-controller.snapshot-controller.svc.cluster.local:8082"), "The callback host for distributed mode")
	flag.StringVar(&distributedWorkerImage, "distributed-worker-image", envOrDefaultValue("DISTRIBUTED_WORKER_IMAGE", "ghcr.io/kaidotio/hippocampus/snapshot-controller/snapshot-worker:main"), "The image to use for the distributed worker jobs")
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

	m, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress:   metricsAddr,
			SecureServing: secureMetrics,
			TLSOpts:       tlsOpts,
		},
		HealthProbeBindAddress: probeAddr,
		WebhookServer: webhook.NewServer(webhook.Options{
			TLSOpts: tlsOpts,
		}),
		LeaderElection:   enableLeaderElection,
		LeaderElectionID: "snapshot-controller",
	})
	if err != nil {
		entrypointLogger.Error(err, "unable to create manager")
		os.Exit(1)
	}

	ctx := context.Background()

	config := capture.DefaultPlaywrightConfig()
	config.ChromeDevtoolsProtocolURL = os.Getenv("CHROME_DEVTOOLS_PROTOCOL_URL")
	if display := os.Getenv("DISPLAY"); display != "" {
		config.Headless = false
	}

	capturer, err := capture.NewPlaywrightCapturer(ctx, config)
	if err != nil {
		entrypointLogger.Error(err, "unable to create screenshot capturer")
		os.Exit(1)
	}

	s3, err := storage.NewS3Storage(ctx, storage.S3Config{
		Bucket: os.Getenv("S3_BUCKET"),
	})
	if err != nil {
		entrypointLogger.Error(err, "unable to create S3 storage backend")
		os.Exit(1)
	}

	if err := (&controllers.SnapshotReconciler{
		Client:                  m.GetClient(),
		Scheme:                  m.GetScheme(),
		Log:                     ctrl.Log.WithName("controllers").WithName("snapshot"),
		Recorder:                m.GetEventRecorderFor("snapshot-controller"),
		Capturer:                capturer,
		Storage:                 s3,
		Distributed:             distributed,
		DistributedCallbackHost: distributedCallbackHost,
	}).SetupWithManager(m); err != nil {
		entrypointLogger.Error(err, "unable to create controller", "controller", "Snapshot")
		os.Exit(1)
	}

	if err := (&controllers.ScheduledSnapshotReconciler{
		Client:                  m.GetClient(),
		Scheme:                  m.GetScheme(),
		Log:                     ctrl.Log.WithName("controllers").WithName("scheduledsnapshot"),
		Recorder:                m.GetEventRecorderFor("scheduledsnapshot-controller"),
		Capturer:                capturer,
		Storage:                 s3,
		Distributed:             distributed,
		DistributedCallbackHost: distributedCallbackHost,
	}).SetupWithManager(m); err != nil {
		entrypointLogger.Error(err, "unable to create controller", "controller", "ScheduledSnapshot")
		os.Exit(1)
	}

	if err := m.Add(runnable.NewServer(s3)); err != nil {
		entrypointLogger.Error(err, "unable to add Server runnable")
		os.Exit(1)
	}

	if err := m.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		entrypointLogger.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := m.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		entrypointLogger.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	entrypointLogger.Info("starting manager")
	if err := m.Start(ctx); err != nil {
		entrypointLogger.Error(err, "problem running manager")
		os.Exit(1)
	}
}
