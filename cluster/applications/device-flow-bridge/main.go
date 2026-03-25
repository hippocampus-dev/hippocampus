package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
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

	"device-flow-bridge/internal/encryption"
	"device-flow-bridge/internal/provider"

	otelpyroscope "github.com/grafana/otel-profiling-go"
	"github.com/grafana/pyroscope-go"
	pyroscopepprof "github.com/grafana/pyroscope-go/http/pprof"
	"github.com/joho/godotenv"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
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

func generateCode() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", xerrors.Errorf("failed to generate random bytes: %w", err)
	}
	return hex.EncodeToString(b), nil
}

type deviceState struct {
	Status   string `json:"status"`
	Scope    string `json:"scope"`
	Token    string `json:"token,omitempty"`
	Interval int    `json:"interval"`
}

var Debug = false

var providers = map[string]provider.Provider{
	"slack": &provider.Slack{},
}

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
	var clientID string
	var clientSecret string
	var callbackURL string
	var redisAddress string
	var deviceCodeTTL time.Duration
	flag.StringVar(&address, "address", envOrDefaultValue("ADDRESS", "0.0.0.0:8080"), "")

	flag.DurationVar(&terminationGracePeriod, "termination-grace-period", envOrDefaultValue("TERMINATION_GRACE_PERIOD", 10*time.Second), "The duration the application needs to terminate gracefully")
	flag.DurationVar(&lameduck, "lameduck", envOrDefaultValue("LAMEDUCK", 1*time.Second), "A period that explicitly asks clients to stop sending requests, although the backend task is listening on that port and can provide the service")
	flag.BoolVar(&keepAlive, "http-keepalive", envOrDefaultValue("HTTP_KEEPALIVE", true), "")
	flag.IntVar(&maxConnections, "max-connections", envOrDefaultValue("MAX_CONNECTIONS", 65532), "Maximum number of connections")

	flag.StringVar(&clientID, "client-id", envOrDefaultValue("CLIENT_ID", ""), "OAuth client ID")
	flag.StringVar(&clientSecret, "client-secret", envOrDefaultValue("CLIENT_SECRET", ""), "OAuth client secret")
	flag.StringVar(&callbackURL, "callback-url", envOrDefaultValue("CALLBACK_URL", ""), "OAuth callback URL (e.g. https://device-flow-bridge.example.com/callback)")
	flag.StringVar(&redisAddress, "redis-address", envOrDefaultValue("REDIS_ADDRESS", "localhost:6379"), "Redis address")
	flag.DurationVar(&deviceCodeTTL, "device-code-ttl", envOrDefaultValue("DEVICE_CODE_TTL", 10*time.Minute), "Device code TTL")
	flag.Parse()

	if clientID == "" {
		log.Fatal("--client-id or CLIENT_ID is required")
	}
	if clientSecret == "" {
		log.Fatal("--client-secret or CLIENT_SECRET is required")
	}
	if callbackURL == "" {
		log.Fatal("--callback-url or CALLBACK_URL is required")
	}
	args := flag.Args()
	if len(args) != 1 {
		log.Fatal("usage: device-flow-bridge <provider>")
	}
	providerName := args[0]

	p, ok := providers[providerName]
	if !ok {
		log.Fatalf("unknown provider: %s", providerName)
	}

	ctx := context.Background()

	redisClient := redis.NewClient(&redis.Options{
		Addr:         redisAddress,
		PoolSize:     10,
		ReadTimeout:  -1,
		WriteTimeout: -1,
	})
	defer func() {
		_ = redisClient.Close()
	}()

	runtime.SetMutexProfileFraction(1)
	runtime.SetBlockProfileRate(1)

	profiler, err := pyroscope.Start(pyroscope.Config{
		ApplicationName: "device-flow-bridge",
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
	meter := sdkmetric.NewMeterProvider(sdkmetric.WithReader(exporter)).Meter("device-flow-bridge")
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

	// https://www.rfc-editor.org/rfc/rfc8628#section-3.1 (Request)
	// https://www.rfc-editor.org/rfc/rfc8628#section-3.2 (Response)
	mux.HandleFuncWithMiddleware("POST /device/code", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		// https://www.rfc-editor.org/rfc/rfc8628#section-3.1
		if r.FormValue("client_id") == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": "invalid_request",
			})
			return
		}

		scope := r.FormValue("scope")

		deviceCode, err := generateCode()
		if err != nil {
			slog.Error("failed to generate device code", "error", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		hashedCode := hashedKey(deviceCode)

		// https://www.rfc-editor.org/rfc/rfc8628#section-5.2
		oauthState, err := generateCode()
		if err != nil {
			slog.Error("failed to generate oauth state", "error", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		ds := deviceState{Status: "pending", Scope: scope, Interval: 5}
		stateJSON, err := json.Marshal(ds)
		if err != nil {
			slog.Error("failed to marshal state", "error", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		pipe := redisClient.Pipeline()
		pipe.Set(r.Context(), redisKey(hashedCode), string(stateJSON), deviceCodeTTL)
		pipe.Set(r.Context(), oauthStateKey(oauthState), deviceCode, deviceCodeTTL)
		if _, err := pipe.Exec(r.Context()); err != nil {
			slog.Error("failed to store device code", "error", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		authURL := p.AuthorizationURL(clientID, scope, oauthState, callbackURL)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"device_code":      deviceCode,
			"verification_uri": authURL,
			"expires_in":       int(deviceCodeTTL.Seconds()),
			"interval":         5,
		})
	})

	mux.HandleFuncWithMiddleware("GET /callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		oauthState := r.URL.Query().Get("state")

		if code == "" || oauthState == "" {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		deviceCode, err := redisClient.GetDel(r.Context(), oauthStateKey(oauthState)).Result()
		if err != nil {
			slog.Error("failed to resolve oauth state", "error", err)
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		hashedCode := hashedKey(deviceCode)

		stored, err := redisClient.Get(r.Context(), redisKey(hashedCode)).Result()
		if err != nil {
			slog.Error("failed to get device code", "error", err)
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		var ds deviceState
		if err := json.Unmarshal([]byte(stored), &ds); err != nil {
			slog.Error("failed to unmarshal state", "error", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		if ds.Status != "pending" {
			http.Error(w, http.StatusText(http.StatusConflict), http.StatusConflict)
			return
		}

		tokenResponse, err := p.ExchangeCode(r.Context(), clientID, clientSecret, code, callbackURL)
		if err != nil {
			slog.Error("failed to exchange code", "error", err)

			ds.Status = "error"
			if updatedJSON, marshalErr := json.Marshal(ds); marshalErr == nil {
				redisClient.Set(r.Context(), redisKey(hashedCode), string(updatedJSON), deviceCodeTTL)
			}

			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		// https://www.rfc-editor.org/rfc/rfc6749#section-5.1
		normalizedToken, err := p.NormalizeTokenResponse(tokenResponse, ds.Scope)
		if err != nil {
			slog.Error("failed to normalize token response", "error", err)

			ds.Status = "error"
			if updatedJSON, marshalErr := json.Marshal(ds); marshalErr == nil {
				redisClient.Set(r.Context(), redisKey(hashedCode), string(updatedJSON), deviceCodeTTL)
			}

			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		tokenJSON, err := json.Marshal(normalizedToken)
		if err != nil {
			slog.Error("failed to marshal token response", "error", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		encryptedToken, err := encryption.Encrypt(deviceCode, tokenJSON)
		if err != nil {
			slog.Error("failed to encrypt token", "error", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		ds.Status = "complete"
		ds.Token = base64.StdEncoding.EncodeToString(encryptedToken)
		updatedJSON, err := json.Marshal(ds)
		if err != nil {
			slog.Error("failed to marshal updated state", "error", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		if err := redisClient.Set(r.Context(), redisKey(hashedCode), string(updatedJSON), deviceCodeTTL).Err(); err != nil {
			slog.Error("failed to store token", "error", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Authorization successful. You can close this window."))
	})

	// https://www.rfc-editor.org/rfc/rfc8628#section-3.4 (Request)
	// https://www.rfc-editor.org/rfc/rfc8628#section-3.5 (Response)
	mux.HandleFuncWithMiddleware("POST /token", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		// https://www.rfc-editor.org/rfc/rfc8628#section-3.4
		grantType := r.FormValue("grant_type")
		deviceCode := r.FormValue("device_code")
		if grantType != "urn:ietf:params:oauth:grant-type:device_code" || deviceCode == "" || r.FormValue("client_id") == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": "invalid_request",
			})
			return
		}

		hashedCode := hashedKey(deviceCode)

		stored, err := redisClient.Get(r.Context(), redisKey(hashedCode)).Result()
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": "expired_token",
			})
			return
		}

		var ds deviceState
		if err := json.Unmarshal([]byte(stored), &ds); err != nil {
			slog.Error("failed to unmarshal state", "error", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		// https://www.rfc-editor.org/rfc/rfc8628#section-3.5
		pollKey := redisKey(hashedCode) + ":last_poll"
		interval := ds.Interval
		if interval <= 0 {
			interval = 5
		}
		if lastPoll, err := redisClient.Get(r.Context(), pollKey).Int64(); err == nil {
			if time.Now().Unix()-lastPoll < int64(interval) {
				ds.Interval = interval + 5
				updatedJSON, err := json.Marshal(ds)
				if err == nil {
					redisClient.Set(r.Context(), redisKey(hashedCode), string(updatedJSON), deviceCodeTTL)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"error":    "slow_down",
					"interval": ds.Interval,
				})
				return
			}
		}
		redisClient.Set(r.Context(), pollKey, time.Now().Unix(), deviceCodeTTL)

		switch ds.Status {
		case "pending":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": "authorization_pending",
			})
			return
		case "error":
			if err := redisClient.Del(r.Context(), redisKey(hashedCode), pollKey).Err(); err != nil {
				slog.Warn("failed to delete device code", "error", err)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": "server_error",
			})
			return
		}

		encryptedBytes, err := base64.StdEncoding.DecodeString(ds.Token)
		if err != nil {
			slog.Error("failed to decode token", "error", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		decryptedToken, err := encryption.Decrypt(deviceCode, encryptedBytes)
		if err != nil {
			slog.Error("failed to decrypt token", "error", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		if err := redisClient.Del(r.Context(), redisKey(hashedCode), pollKey).Err(); err != nil {
			slog.Warn("failed to delete device code", "error", err)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(decryptedToken)
	})

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

func hashedKey(deviceCode string) string {
	h := sha256.Sum256([]byte(deviceCode))
	return hex.EncodeToString(h[:])
}

func redisKey(deviceCode string) string {
	return fmt.Sprintf("device-flow-bridge:device:%s", deviceCode)
}

func oauthStateKey(state string) string {
	return fmt.Sprintf("device-flow-bridge:oauth-state:%s", state)
}
