package main

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"github-token-server/internal/types"
	"io"
	"log"
	"log/slog"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"slices"
	"strconv"
	"syscall"
	"time"

	"github.com/golang-jwt/jwt/v5"
	otelpyroscope "github.com/grafana/otel-profiling-go"
	"github.com/grafana/pyroscope-go"
	pyroscopepprof "github.com/grafana/pyroscope-go/http/pprof"
	"github.com/joho/godotenv"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	otelprometheus "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/net/netutil"
	"golang.org/x/xerrors"
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

type Middleware func(http.Handler) http.Handler

type myRouter struct {
	*http.ServeMux
	logger                           *slog.Logger
	httpRequestsDurationMicroSeconds metric.Int64Histogram
	middlewares                      []Middleware
}

func (m *myRouter) Use(middleware ...Middleware) {
	m.middlewares = append(m.middlewares, middleware...)
}

func (m *myRouter) HandleWithMiddleware(pattern string, handler http.Handler) {
	m.ServeMux.Handle(pattern, m.middleware(pattern, handler))
}

func (m *myRouter) HandleFuncWithMiddleware(pattern string, handler http.HandlerFunc) {
	m.ServeMux.Handle(pattern, m.middleware(pattern, handler))
}

func (m *myRouter) middleware(pattern string, next http.Handler) http.Handler {
	var handler http.Handler

	handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		span := trace.SpanFromContext(r.Context())
		contextLogger := m.logger.With(
			slog.String("traceid", span.SpanContext().TraceID().String()),
			slog.String("spanid", span.SpanContext().SpanID().String()),
		)

		slog.SetDefault(contextLogger)

		defer func() {
			if err := recover(); err != nil {
				slog.Error("panic occurred", "error", err, "stack", string(debug.Stack()))
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}

			if err := r.Context().Err(); errors.Is(err, context.Canceled) {
				slog.Debug("client closed connection")
			}
		}()

		pyroscope.TagWrapper(r.Context(), pyroscope.Labels(), func(ctx context.Context) {
			now := time.Now()
			next.ServeHTTP(w, r)
			m.httpRequestsDurationMicroSeconds.Record(ctx, time.Since(now).Microseconds(), metric.WithAttributes(
				attribute.Key("method").String(r.Method),
				attribute.Key("handler").String(pattern),
			))
		})
	})

	for _, middleware := range slices.Backward(m.middlewares) {
		handler = middleware(handler)
	}

	return otelhttp.NewHandler(handler, pattern, otelhttp.WithSpanNameFormatter(func(operation string, r *http.Request) string {
		return fmt.Sprintf("%s %s", r.Method, operation)
	}), otelhttp.WithMetricAttributesFn(func(r *http.Request) []attribute.KeyValue {
		return []attribute.KeyValue{}
	}))
}

var Debug = false

func main() {
	http.DefaultTransport.(*http.Transport).MaxIdleConnsPerHost = http.DefaultTransport.(*http.Transport).MaxIdleConns

	if Debug {
		_ = godotenv.Load()
	}

	var address string
	var terminationGracePeriod time.Duration
	var lameduck time.Duration
	var keepAlive bool
	var maxConnections int
	var clientId string
	var privateKey string
	flag.StringVar(&address, "address", envOrDefaultValue("ADDRESS", "0.0.0.0:8080"), "")

	flag.DurationVar(&terminationGracePeriod, "termination-grace-period", envOrDefaultValue("TERMINATION_GRACE_PERIOD", 10*time.Second), "The duration the application needs to terminate gracefully")
	flag.DurationVar(&lameduck, "lameduck", envOrDefaultValue("LAMEDUCK", 1*time.Second), "A period that explicitly asks clients to stop sending requests, although the backend task is listening on that port and can provide the service")
	flag.BoolVar(&keepAlive, "http-keepalive", envOrDefaultValue("HTTP_KEEPALIVE", true), "")
	flag.IntVar(&maxConnections, "max-connections", envOrDefaultValue("MAX_CONNECTIONS", 65532), "Maximum number of connections")

	flag.StringVar(&clientId, "client-id", envOrDefaultValue("CLIENT_ID", ""), "Client ID of the GitHub App")
	flag.StringVar(&privateKey, "private-key", envOrDefaultValue("PRIVATE_KEY", ""), "Private key of the GitHub App")
	flag.Parse()

	ctx := context.Background()

	runtime.SetMutexProfileFraction(1)
	runtime.SetBlockProfileRate(1)

	profiler, err := pyroscope.Start(pyroscope.Config{
		ApplicationName: "github-token-server",
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
		log.Fatalf("failed to create profiler: %+v", err)
	}

	otel.SetTextMapPropagator(propagation.TraceContext{})

	r, err := sdkresource.Merge(
		sdkresource.Default(),
		sdkresource.NewWithAttributes(semconv.SchemaURL),
	)
	if err != nil {
		log.Fatalf("failed to create resource: %+v", err)
	}
	traceExporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		log.Fatalf("failed to create trace exporter: %+v", err)
	}
	traceProvider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(r),
		sdktrace.WithBatcher(traceExporter),
	)
	otel.SetTracerProvider(otelpyroscope.NewTracerProvider(traceProvider))

	exporter, err := otelprometheus.New()
	if err != nil {
		log.Fatalf("failed to create exporter: %+v", err)
	}
	// NOTE: Gauge(UpDownCounter), Summary or Untyped does not support exemplars
	// https://github.com/prometheus/client_golang/blob/v1.20.4/prometheus/metric.go#L200
	meter := sdkmetric.NewMeterProvider(sdkmetric.WithReader(exporter)).Meter("github-token-server")
	httpRequestsDurationMicroSeconds, err := meter.Int64Histogram("http_requests_duration_micro_seconds")
	if err != nil {
		log.Fatalf("failed to create histogram: %+v", err)
	}

	logLevel := slog.LevelInfo
	if v, ok := os.LookupEnv("GO_LOG"); ok {
		if err := logLevel.UnmarshalText([]byte(v)); err != nil {
			log.Fatalf("failed to parse log level: %+v", err)
		}
	}
	handlerOpts := &slog.HandlerOptions{
		Level: logLevel,
		// https://opentelemetry.io/docs/specs/otel/logs/data-model/
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

	mux := myRouter{http.NewServeMux(), logger, httpRequestsDurationMicroSeconds, []Middleware{}}
	routes := []struct {
		pattern                  string
		installationURLGenerator func(r *http.Request) string
		bodyGenerator            func(r *http.Request) io.ReadCloser
	}{
		{"POST /users/{username}/access_tokens", func(r *http.Request) string {
			return fmt.Sprintf("https://api.github.com/users/%s/installation", r.PathValue("username"))
		}, func(r *http.Request) io.ReadCloser {
			var m types.Body

			_ = json.NewDecoder(r.Body).Decode(&m)

			if p := r.URL.Query().Get("profile"); p != "" {
				m.ResolveProfile(p)
			}

			b, err := json.Marshal(m)
			if err != nil {
				return r.Body
			}
			return io.NopCloser(bytes.NewReader(b))
		}},
		{"POST /orgs/{org}/access_tokens", func(r *http.Request) string {
			return fmt.Sprintf("https://api.github.com/orgs/%s/installation", r.PathValue("org"))
		}, func(r *http.Request) io.ReadCloser {
			var m types.Body

			_ = json.NewDecoder(r.Body).Decode(&m)

			if p := r.URL.Query().Get("profile"); p != "" {
				m.ResolveProfile(p)
			}

			b, err := json.Marshal(m)
			if err != nil {
				return r.Body
			}
			return io.NopCloser(bytes.NewReader(b))
		}},
		{"POST /repos/{owner}/{repo}/access_tokens", func(r *http.Request) string {
			return fmt.Sprintf("https://api.github.com/repos/%s/%s/installation", r.PathValue("owner"), r.PathValue("repo"))
		}, func(r *http.Request) io.ReadCloser {
			var m types.Body

			_ = json.NewDecoder(r.Body).Decode(&m)

			if p := r.URL.Query().Get("profile"); p != "" {
				m.ResolveProfile(p)
			}

			// Restrict the repositories that the token can access
			m.Repositories = []string{r.PathValue("repo")}

			b, err := json.Marshal(m)
			if err != nil {
				return r.Body
			}

			return io.NopCloser(bytes.NewReader(b))
		}},
	}
	for _, route := range routes {
		mux.HandleFuncWithMiddleware(route.pattern, func(w http.ResponseWriter, r *http.Request) {
			err, jwtToken := signJwt(privateKey, clientId)
			if err != nil {
				slog.Error("failed to sign jwt", "error", err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			installationRequest, err := http.NewRequestWithContext(r.Context(), http.MethodGet, route.installationURLGenerator(r), nil)
			if err != nil {
				slog.Error("failed to create request", "error", err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			installationRequest.Header.Set("Accept", "application/vnd.github+json")
			installationRequest.Header.Set("Authorization", fmt.Sprintf("Bearer %s", *jwtToken))
			installationRequest.Header.Set("X-GitHub-Api-Version", "2022-11-28")
			installationResponse, err := http.DefaultClient.Do(installationRequest)
			if err != nil {
				slog.Error("failed to do request", "error", err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			defer func() {
				_ = installationResponse.Body.Close()
			}()

			if installationResponse.StatusCode >= http.StatusBadRequest {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(installationResponse.StatusCode)
				_, _ = io.Copy(w, installationResponse.Body)
				return
			}

			installation := struct {
				ID int `json:"id"`
			}{}
			if err := json.NewDecoder(installationResponse.Body).Decode(&installation); err != nil {
				slog.Error("failed to decode installation", "error", err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			accessTokenRequest, err := http.NewRequestWithContext(r.Context(), http.MethodPost, fmt.Sprintf("https://api.github.com/app/installations/%d/access_tokens", installation.ID), route.bodyGenerator(r))
			if err != nil {
				slog.Error("failed to create request", "error", err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			accessTokenRequest.Header.Set("Accept", "application/vnd.github+json")
			accessTokenRequest.Header.Set("Authorization", fmt.Sprintf("Bearer %s", *jwtToken))
			accessTokenRequest.Header.Set("X-GitHub-Api-Version", "2022-11-28")
			accessTokenResponse, err := http.DefaultClient.Do(accessTokenRequest)
			if err != nil {
				slog.Error("failed to do request", "error", err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			defer func() {
				_ = accessTokenResponse.Body.Close()
			}()

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(accessTokenResponse.StatusCode)
			_, _ = io.Copy(w, accessTokenResponse.Body)
		})
	}

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

	listener, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("failed to listen: %+v", err)
	}

	server := &http.Server{
		Handler: mux,
	}
	server.SetKeepAlivesEnabled(keepAlive)

	go func() {
		if err := server.Serve(netutil.LimitListener(listener, maxConnections)); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("failed to listen: %+v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM)
	<-quit
	time.Sleep(lameduck)

	ctx, cancel := context.WithTimeout(ctx, terminationGracePeriod)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("failed to shutdown: %+v", err)
	}

	if err := traceProvider.Shutdown(ctx); err != nil {
		log.Fatalf("failed to shutdown trace provider: %+v", err)
	}

	if err := profiler.Stop(); err != nil {
		log.Fatalf("failed to shutdown profiler: %+v", err)
	}
}

func signJwt(privateKey string, clientId string) (error, *string) {
	block, _ := pem.Decode([]byte(privateKey))
	if block == nil {
		return xerrors.New("failed to decode private key"), nil
	}

	rsaPrivateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return xerrors.Errorf("failed to parse private key: %w", err), nil
	}

	now := time.Now()
	claims := jwt.MapClaims{
		"iat": now.Unix(),
		"exp": now.Add(time.Minute * 10).Unix(),
		"iss": clientId,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	jwtToken, err := token.SignedString(rsaPrivateKey)
	if err != nil {
		return xerrors.Errorf("failed to sign token: %w", err), nil
	}
	return nil, &jwtToken
}
