package main

import (
	"context"
	"crypto/tls"
	ingressv1 "dedicated-container-ingress-controller/api/v1"
	"dedicated-container-ingress-controller/internal/controllers"
	"dedicated-container-ingress-controller/internal/factory"
	"dedicated-container-ingress-controller/internal/gc"
	"dedicated-container-ingress-controller/internal/proxy"
	"flag"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
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

	"k8s.io/client-go/kubernetes"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(ingressv1.AddToScheme(scheme))
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
	var proxyAddress string
	var redisAddress string
	var cookieSecretKey string
	var cookieMaxAge int
	var podsLimit int64
	var gcInterval time.Duration
	var gcLifetime time.Duration
	flag.StringVar(&metricsAddr, "metrics-bind-address", envOrDefaultValue("METRICS_BIND_ADDRESS", "0.0.0.0:8080"), "The address the metric endpoint binds to.")
	flag.BoolVar(&secureMetrics, "metrics-secure", envOrDefaultValue("METRICS_SECURE", false), "If set the metrics endpoint is served securely")
	flag.BoolVar(&enableHTTP2, "enable-http2", envOrDefaultValue("ENABLE_HTTP2", false), "If set, HTTP/2 will be enabled for the metrics and webhook servers")
	flag.StringVar(&probeAddr, "health-probe-bind-address", envOrDefaultValue("HEALTH_PROBE_BIND_ADDRESS", "0.0.0.0:8081"), "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", envOrDefaultValue("ENABLE_LEADER_ELECTION", false), "Enable leader election for controller manager.")
	flag.StringVar(&proxyAddress, "proxy-address", envOrDefaultValue("PROXY_ADDRESS", "0.0.0.0:8000"), "The address the proxy server binds to.")
	flag.StringVar(&redisAddress, "redis-address", envOrDefaultValue("REDIS_ADDRESS", "127.0.0.1:6379"), "The address of the Redis server.")
	flag.StringVar(&cookieSecretKey, "cookie-secret-key", envOrDefaultValue("COOKIE_SECRET_KEY", ""), "Secret key for signing session cookies.")
	flag.IntVar(&cookieMaxAge, "cookie-max-age", envOrDefaultValue("COOKIE_MAX_AGE", 86400), "Max age of session cookies in seconds.")
	flag.Int64Var(&podsLimit, "pods-limit", envOrDefaultValue("PODS_LIMIT", int64(30)), "Maximum number of dedicated pods.")
	flag.DurationVar(&gcInterval, "gc-interval", envOrDefaultValue("GC_INTERVAL", 1*time.Minute), "Interval between garbage collection runs.")
	flag.DurationVar(&gcLifetime, "gc-lifetime", envOrDefaultValue("GC_LIFETIME", 1*time.Hour), "Pod idle lifetime before garbage collection.")
	opts := zap.Options{}
	opts.BindFlags(flag.CommandLine)
	klog.InitFlags(flag.CommandLine)
	flag.Parse()

	http.DefaultTransport.(*http.Transport).MaxIdleConnsPerHost = http.DefaultTransport.(*http.Transport).MaxIdleConns

	zapLogger := zap.New(zap.UseFlagOptions(&opts))
	klog.SetLogger(zapLogger)
	ctrl.SetLogger(zapLogger)

	entrypointLogger := ctrl.Log.WithName("entrypoint")

	if cookieSecretKey == "" {
		entrypointLogger.Error(nil, "--cookie-secret-key or COOKIE_SECRET_KEY is required")
		os.Exit(1)
	}

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
		LeaderElectionID: "dedicated-container-ingress-controller",
	})
	if err != nil {
		entrypointLogger.Error(err, "unable to create manager")
		os.Exit(1)
	}

	clientset, err := kubernetes.NewForConfig(m.GetConfig())
	if err != nil {
		entrypointLogger.Error(err, "unable to create clientset")
		os.Exit(1)
	}

	dedicatedContainerFactory := factory.NewDedicatedContainerFactory(clientset)

	redisClient := redis.NewClient(&redis.Options{
		Addr: redisAddress,
	})

	if err := (&controllers.DedicatedContainerIngressReconciler{
		Client:   m.GetClient(),
		Scheme:   m.GetScheme(),
		Log:      ctrl.Log.WithName("controllers").WithName("DedicatedContainerIngress"),
		Recorder: m.GetEventRecorderFor("dedicated-container-ingress-controller"),
		Factory:  dedicatedContainerFactory,
	}).SetupWithManager(m); err != nil {
		entrypointLogger.Error(err, "unable to create controller", "controller", "DedicatedContainerIngress")
		os.Exit(1)
	}

	proxyServer := proxy.NewServer(&proxy.Args{
		Address:         proxyAddress,
		CookieSecretKey: cookieSecretKey,
		CookieMaxAge:    cookieMaxAge,
		PodsLimit:       podsLimit,
		RedisClient:     redisClient,
		Factory:         dedicatedContainerFactory,
	})

	garbageCollector := gc.NewGarbageCollector(gcInterval, gcLifetime, redisClient, clientset)

	if err := m.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		entrypointLogger.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := m.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		entrypointLogger.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	if err := proxyServer.Start(); err != nil {
		entrypointLogger.Error(err, "unable to start proxy server")
		os.Exit(1)
	}
	go garbageCollector.Start()

	entrypointLogger.Info("starting manager")
	if err := m.Start(ctrl.SetupSignalHandler()); err != nil {
		entrypointLogger.Error(err, "problem running manager")
		os.Exit(1)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := proxyServer.Shutdown(shutdownCtx); err != nil {
		entrypointLogger.Error(err, "failed to shutdown proxy server")
	}
	garbageCollector.Stop()
	if err := redisClient.Close(); err != nil {
		entrypointLogger.Error(err, "failed to close redis client")
	}
}
