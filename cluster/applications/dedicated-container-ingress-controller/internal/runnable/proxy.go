package runnable

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"dedicated-container-ingress-controller/internal/factory"
	"dedicated-container-ingress-controller/internal/myhttp"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/http/pprof"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	otelpyroscope "github.com/grafana/otel-profiling-go"
	"github.com/grafana/pyroscope-go"
	pyroscopepprof "github.com/grafana/pyroscope-go/http/pprof"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	otelprometheus "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"golang.org/x/net/netutil"
	"golang.org/x/sync/singleflight"
	"golang.org/x/xerrors"
	corev1 "k8s.io/api/core/v1"
)

type Server struct {
	address                string
	terminationGracePeriod time.Duration
	lameduck               time.Duration
	keepAlive              bool
	maxConnections         int
	secretKey              []byte
	cookieMaxAge           int
	podsLimit              int64
	redisClient            *redis.Client
	factory                *factory.DedicatedContainerFactory
	transport              http.RoundTripper
	group                  singleflight.Group
}

func NewServer(redisClient *redis.Client, dedicatedContainerFactory *factory.DedicatedContainerFactory) *Server {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConnsPerHost = transport.MaxIdleConns

	return &Server{
		address:                envOrDefaultValue("ADDRESS", "0.0.0.0:8000"),
		terminationGracePeriod: envOrDefaultValue("TERMINATION_GRACE_PERIOD", 10*time.Second),
		lameduck:               envOrDefaultValue("LAMEDUCK", 1*time.Second),
		keepAlive:              envOrDefaultValue("HTTP_KEEPALIVE", true),
		maxConnections:         envOrDefaultValue("MAX_CONNECTIONS", 65532),
		secretKey:              []byte(envOrDefaultValue("COOKIE_SECRET_KEY", "")),
		cookieMaxAge:           envOrDefaultValue("COOKIE_MAX_AGE", 86400),
		podsLimit:              envOrDefaultValue("PODS_LIMIT", int64(30)),
		redisClient:            redisClient,
		factory:                dedicatedContainerFactory,
		transport:              otelhttp.NewTransport(transport),
	}
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

func (s *Server) NeedLeaderElection() bool {
	return false
}

var Debug = false

func (s *Server) Start(ctx context.Context) error {
	if len(s.secretKey) == 0 {
		return xerrors.New("COOKIE_SECRET_KEY is required")
	}

	runtime.SetMutexProfileFraction(1)
	runtime.SetBlockProfileRate(1)

	profiler, err := pyroscope.Start(pyroscope.Config{
		ApplicationName: "dedicated-container-ingress-controller",
		ServerAddress:   os.Getenv("PYROSCOPE_ENDPOINT"),
		UploadRate:      60 * time.Second,
		ProfileTypes: []pyroscope.ProfileType{
			pyroscope.ProfileCPU,
			pyroscope.ProfileAllocObjects,
			pyroscope.ProfileAllocSpace,
			pyroscope.ProfileInuseObjects,
			pyroscope.ProfileInuseSpace,
			pyroscope.ProfileGoroutines,
			pyroscope.ProfileMutexCount,
			pyroscope.ProfileMutexDuration,
			pyroscope.ProfileBlockCount,
			pyroscope.ProfileBlockDuration,
		},
	})
	if err != nil {
		return xerrors.Errorf("failed to create profiler: %w", err)
	}

	otel.SetTextMapPropagator(propagation.TraceContext{})

	r := sdkresource.Default()
	traceExporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		return xerrors.Errorf("failed to create trace exporter: %w", err)
	}
	traceProvider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(r),
		sdktrace.WithBatcher(traceExporter),
	)
	otel.SetTracerProvider(otelpyroscope.NewTracerProvider(traceProvider))

	exporter, err := otelprometheus.New()
	if err != nil {
		return xerrors.Errorf("failed to create exporter: %w", err)
	}
	meter := sdkmetric.NewMeterProvider(sdkmetric.WithReader(exporter)).Meter("dedicated-container-ingress-controller")
	httpRequestsDurationMicroSeconds, err := meter.Int64Histogram("http_requests_duration_micro_seconds")
	if err != nil {
		return xerrors.Errorf("failed to create histogram: %w", err)
	}

	logLevel := slog.LevelInfo
	if v, ok := os.LookupEnv("GO_LOG"); ok {
		if err := logLevel.UnmarshalText([]byte(v)); err != nil {
			return xerrors.Errorf("failed to parse log level: %w", err)
		}
	}
	handlerOpts := &slog.HandlerOptions{
		Level: logLevel,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			switch a.Key {
			case slog.LevelKey:
				a.Key = "severitytext"
			case slog.MessageKey:
				a.Key = "body"
			}
			return a
		},
	}
	logger := slog.New(slog.NewJSONHandler(os.Stderr, handlerOpts))
	if Debug {
		logger = slog.New(slog.NewTextHandler(os.Stderr, handlerOpts))
	}

	mux := myhttp.NewServerMux(logger, httpRequestsDurationMicroSeconds)

	mux.HandleFuncWithMiddleware("/", s.handleProxy)

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(http.StatusText(http.StatusOK)))
	})

	mux.Handle("GET /metrics", promhttp.InstrumentMetricHandler(
		prometheus.DefaultRegisterer, promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{
			EnableOpenMetrics: true,
		}),
	))

	if Debug {
		mux.HandleFunc("GET /debug/pprof/", pprof.Index)
		mux.HandleFunc("GET /debug/pprof/cmdline", pprof.Cmdline)
		mux.HandleFunc("GET /debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("GET /debug/pprof/trace", pprof.Trace)
		mux.HandleFunc("GET /debug/pprof/profile", pyroscopepprof.Profile)
	}

	listener, err := net.Listen("tcp", s.address)
	if err != nil {
		return xerrors.Errorf("failed to listen on address %s: %w", s.address, err)
	}

	server := &http.Server{
		Handler: mux,
	}
	server.SetKeepAlivesEnabled(s.keepAlive)

	go func() {
		if err := server.Serve(netutil.LimitListener(listener, s.maxConnections)); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("failed to serve HTTP", "error", err)
		}
	}()

	<-ctx.Done()
	time.Sleep(s.lameduck)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), s.terminationGracePeriod)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return xerrors.Errorf("failed to shutdown server: %w", err)
	}

	if err := traceProvider.Shutdown(shutdownCtx); err != nil {
		return xerrors.Errorf("failed to shutdown trace provider: %w", err)
	}

	if err := profiler.Stop(); err != nil {
		return xerrors.Errorf("failed to shutdown profiler: %w", err)
	}

	return nil
}

type sessionData struct {
	Identifier string `json:"identifier"`
	Host       string `json:"host"`
}

func (s *Server) handleProxy(w http.ResponseWriter, r *http.Request) {
	key := trimHost(r.Host)

	session, err := s.getSession(r, key)
	if err != nil {
		slog.Error("failed to get session", "error", err)
	}

	var identifier string
	var host string
	reachable := false
	if session != nil {
		identifier = session.Identifier
		host = session.Host
		reachable = checkReachable(host)
	}

	if session == nil || !reachable {
		count, err := s.podsCount(r.Context())
		if err != nil {
			slog.Error("failed to count pods", "error", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		if count >= s.podsLimit {
			http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
			return
		}
		result, err, _ := s.group.Do(key, func() (interface{}, error) {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			return s.factory.Create(ctx, key)
		})
		if err != nil {
			if errors.Is(err, factory.ErrNoBackend) {
				http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
				return
			}
			slog.Error("failed to create pod", "error", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		pod := result.(*corev1.Pod)
		identifier = fmt.Sprintf("%s/%s", pod.Name, pod.Namespace)
		host = fmt.Sprintf("%s:%d", pod.Status.PodIP, getHTTPPort(pod))
		if err := s.saveSession(w, key, &sessionData{
			Identifier: identifier,
			Host:       host,
		}); err != nil {
			slog.Error("failed to save session", "error", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}

	proxy := httputil.NewSingleHostReverseProxy(&url.URL{
		Scheme: "http",
		Host:   host,
	})
	proxy.Transport = s.transport
	proxy.ServeHTTP(w, r)

	if err := s.updateTimestamp(context.Background(), identifier); err != nil {
		slog.Error("failed to update timestamp", "error", err)
	}
}

func (s *Server) podsCount(ctx context.Context) (int64, error) {
	count, err := s.redisClient.ZCard(ctx, "pods").Result()
	if err != nil {
		return 0, xerrors.Errorf("failed to ZCARD: %w", err)
	}
	return count, nil
}

func (s *Server) updateTimestamp(ctx context.Context, identifier string) error {
	if err := s.redisClient.ZAdd(ctx, "pods", redis.Z{
		Score:  float64(time.Now().Unix()),
		Member: identifier,
	}).Err(); err != nil {
		return xerrors.Errorf("failed to ZADD: %w", err)
	}
	return nil
}

func (s *Server) getSession(r *http.Request, name string) (*sessionData, error) {
	cookie, err := r.Cookie(name)
	if err != nil {
		return nil, nil
	}
	parts := strings.SplitN(cookie.Value, ".", 2)
	if len(parts) != 2 {
		return nil, nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, nil
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, nil
	}
	mac := hmac.New(sha256.New, s.secretKey)
	mac.Write(payload)
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return nil, nil
	}
	var data sessionData
	if err := json.Unmarshal(payload, &data); err != nil {
		return nil, nil
	}
	return &data, nil
}

func (s *Server) saveSession(w http.ResponseWriter, name string, data *sessionData) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return xerrors.Errorf("failed to marshal session: %w", err)
	}
	mac := hmac.New(sha256.New, s.secretKey)
	mac.Write(payload)
	value := base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   s.cookieMaxAge,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

func getHTTPPort(pod *corev1.Pod) int32 {
	for _, container := range pod.Spec.Containers {
		for _, port := range container.Ports {
			if port.Name == "http" {
				return port.ContainerPort
			}
		}
	}
	return 80
}

func checkReachable(host string) bool {
	conn, err := net.DialTimeout("tcp", host, time.Second)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func trimHost(host string) string {
	if idx := strings.IndexByte(host, ':'); idx > 0 {
		host = host[:idx]
	}
	return host
}
