package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"gateway-proxy/internal/controllers"
	"gateway-proxy/internal/handler"
	"gateway-proxy/internal/proxy"
	"net/http"
	"os"
	"strconv"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(gatewayv1.Install(scheme))
	utilruntime.Must(gatewayv1alpha2.Install(scheme))
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
	var host string
	var port int
	var certDir string
	var enableLeaderElection bool
	var proxyProtocolVersion string
	flag.StringVar(&metricsAddr, "metrics-bind-address", envOrDefaultValue("METRICS_BIND_ADDRESS", "0.0.0.0:8080"), "The address the metric endpoint binds to.")
	flag.BoolVar(&secureMetrics, "metrics-secure", envOrDefaultValue("METRICS_SECURE", false), "If set the metrics endpoint is served securely")
	flag.BoolVar(&enableHTTP2, "enable-http2", envOrDefaultValue("ENABLE_HTTP2", false), "If set, HTTP/2 will be enabled for the metrics and webhook servers")
	flag.StringVar(&probeAddr, "health-probe-bind-address", envOrDefaultValue("HEALTH_PROBE_BIND_ADDRESS", "0.0.0.0:8081"), "The address the probe endpoint binds to.")
	flag.StringVar(&host, "host", envOrDefaultValue("HOST", ""), "Server host")
	flag.IntVar(&port, "port", envOrDefaultValue("PORT", 9443), "Server port")
	flag.StringVar(&certDir, "certDir", envOrDefaultValue("CERT_DIR", "/var/k8s-webhook-server/serving-certs"), "CertDir is the directory that contains the server key and certificate. The server key and certificate.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", envOrDefaultValue("ENABLE_LEADER_ELECTION", false),
		"Enable leader election for controller manager.")
	flag.StringVar(&proxyProtocolVersion, "proxy-protocol-version", envOrDefaultValue("PROXY_PROTOCOL_VERSION", ""), "PROXY protocol version to send to backends (\"1\" for v1 text, \"2\" for v2 binary, \"\" to disable).")
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
			Host:    host,
			Port:    port,
			CertDir: certDir,
		}),
		LeaderElection:   enableLeaderElection,
		LeaderElectionID: "gateway-proxy",
	})
	if err != nil {
		entrypointLogger.Error(err, "unable to create manager")
		os.Exit(1)
	}

	version := proxy.ProtocolVersion(proxyProtocolVersion)
	switch version {
	case proxy.ProtocolDisabled, proxy.ProtocolV1, proxy.ProtocolV2:
	default:
		entrypointLogger.Error(fmt.Errorf("unsupported value: %q", proxyProtocolVersion), "--proxy-protocol-version must be \"\", \"1\", or \"2\"")
		os.Exit(1)
	}

	proxyManager := proxy.NewManager(ctrl.Log.WithName("proxy"), version)

	controllerName := gatewayv1.GatewayController("kaidotio.github.io/gateway-proxy")

	resolver := &controllers.RouteResolver{
		Reader:         m.GetCache(),
		ControllerName: controllerName,
	}

	if err := (&controllers.GatewayClassController{
		Client:         m.GetClient(),
		Scheme:         m.GetScheme(),
		ControllerName: controllerName,
	}).SetupWithManager(m); err != nil {
		entrypointLogger.Error(err, "unable to create controller", "controller", "GatewayClass")
		os.Exit(1)
	}

	if err := (&controllers.GatewayController{
		Client:         m.GetClient(),
		Scheme:         m.GetScheme(),
		Recorder:       m.GetEventRecorderFor("gateway-proxy"),
		ControllerName: controllerName,
		ProxyManager:   proxyManager,
		Resolver:       resolver,
	}).SetupWithManager(m); err != nil {
		entrypointLogger.Error(err, "unable to create controller", "controller", "Gateway")
		os.Exit(1)
	}

	if err := m.Add(&controllers.ProxyRunner{
		Cache:        m.GetCache(),
		Resolver:     resolver,
		ProxyManager: proxyManager,
		Log:          ctrl.Log.WithName("proxy-runner"),
	}); err != nil {
		entrypointLogger.Error(err, "unable to add proxy runner")
		os.Exit(1)
	}

	webhookServer := m.GetWebhookServer()
	webhookServer.Register("/validate", &webhook.Admission{Handler: &handler.GatewayHandler{Client: m.GetClient(), Decoder: admission.NewDecoder(m.GetScheme()), Resolver: resolver}})

	if err := m.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		entrypointLogger.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	// Ready() is false until the first successful ProxyManager.Update() call
	// from the ProxyRunner. All pods run proxy listeners (ProxyRunner does not
	// require leader election). When no Gateways exist, the pod stays NotReady
	// intentionally: if the pod restarted while proxying traffic, marking it
	// Ready before listeners rebind would cause the LoadBalancer Service to
	// route traffic to a pod with no active listeners.
	if err := m.AddReadyzCheck("readyz", func(_ *http.Request) error {
		if !proxyManager.Ready() {
			return fmt.Errorf("proxy listeners not yet ready")
		}
		return nil
	}); err != nil {
		entrypointLogger.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	entrypointLogger.Info("starting manager")
	if err := m.Start(ctrl.SetupSignalHandler()); err != nil {
		entrypointLogger.Error(err, "problem running manager")
		os.Exit(1)
	}
}
