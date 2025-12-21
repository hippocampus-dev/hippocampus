package main

import (
	"crypto/tls"
	"flag"
	garV1 "github-actions-runner-controller/api/v1"
	"github-actions-runner-controller/internal/controllers"
	"os"
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
	utilruntime.Must(garV1.AddToScheme(scheme))
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
	var pushRegistryURL string
	var pullRegistryURL string
	var githubAppClientId string
	var githubAppInstallationId string
	var githubAppPrivateKey string
	var kanikoImage string
	var binaryVersion string
	var runnerVersion string
	var disableupdate bool
	flag.StringVar(&metricsAddr, "metrics-bind-address", envOrDefaultValue("METRICS_BIND_ADDRESS", "0.0.0.0:8080"), "The address the metric endpoint binds to.")
	flag.BoolVar(&secureMetrics, "metrics-secure", envOrDefaultValue("METRICS_SECURE", false), "If set the metrics endpoint is served securely")
	flag.BoolVar(&enableHTTP2, "enable-http2", envOrDefaultValue("ENABLE_HTTP2", false), "If set, HTTP/2 will be enabled for the metrics and webhook servers")
	flag.StringVar(&probeAddr, "health-probe-bind-address", envOrDefaultValue("HEALTH_PROBE_BIND_ADDRESS", "0.0.0.0:8081"), "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", envOrDefaultValue("ENABLE_LEADER_ELECTION", false),
		"Enable leader election for controller manager.")
	flag.StringVar(&pushRegistryURL, "push-registry-url", envOrDefaultValue("PUSH_REGISTRY_URL", "ghcr.io/hippocampus-dev/hippocampus/github-actions-runner-controller"), "URL of Docker Registry used as push destination.")
	flag.StringVar(&pullRegistryURL, "pull-registry-url", envOrDefaultValue("PULL_REGISTRY_URL", "ghcr.io/hippocampus-dev/hippocampus/github-actions-runner-controller"), "URL of Docker Registry used as pull source.")
	flag.StringVar(&githubAppClientId, "github-app-client-id", envOrDefaultValue("GITHUB_APP_CLIENT_ID", ""), "GitHub App Client ID")
	flag.StringVar(&githubAppInstallationId, "github-app-installation-id", envOrDefaultValue("GITHUB_APP_INSTALLATION_ID", ""), "GitHub App Installation ID")
	flag.StringVar(&githubAppPrivateKey, "github-app-private-key", envOrDefaultValue("GITHUB_APP_PRIVATE_KEY", ""), "GitHub App Private Key")
	flag.StringVar(&kanikoImage, "kaniko-image", envOrDefaultValue("KANIKO_IMAGE", "gcr.io/kaniko-project/executor:v1.23.0"), "Docker Image of kaniko used by builder container")
	flag.StringVar(&binaryVersion, "binary-version", envOrDefaultValue("BINARY_VERSION", "0.1.0"), "Version of own runner binary")
	flag.StringVar(&runnerVersion, "runner-version", envOrDefaultValue("RUNNER_VERSION", "2.323.0"), "Version of GitHub Actions runner")
	flag.BoolVar(&disableupdate, "disableupdate", envOrDefaultValue("DISABLEUPDATE", false), "Disable self-hosted runner automatic update to the latest released version")
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
		LeaderElectionID: "github-actions-runner-controller",
	})
	if err != nil {
		entrypointLogger.Error(err, "unable to create manager")
		os.Exit(1)
	}

	if err := (&controllers.RunnerReconciler{
		Client:                  m.GetClient(),
		Scheme:                  m.GetScheme(),
		Log:                     ctrl.Log.WithName("controllers").WithName("Runner"),
		Recorder:                m.GetEventRecorderFor("github-actions-runner-controller"),
		PushRegistryURL:         pushRegistryURL,
		PullRegistryURL:         pullRegistryURL,
		GitHubAppClientId:       githubAppClientId,
		GitHubAppInstallationId: githubAppInstallationId,
		GitHubAppPrivateKey:     githubAppPrivateKey,
		KanikoImage:             kanikoImage,
		BinaryVersion:           binaryVersion,
		RunnerVersion:           runnerVersion,
		Disableupdate:           disableupdate,
	}).SetupWithManager(m); err != nil {
		entrypointLogger.Error(err, "unable to create controller", "controller", "Runner")
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
	if err := m.Start(ctrl.SetupSignalHandler()); err != nil {
		entrypointLogger.Error(err, "problem running manager")
		os.Exit(1)
	}
}
