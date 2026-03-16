package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"net/http"
	"net/http/pprof"
	"net/url"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

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
	var baseURL string
	var redisAddress string
	var deviceCodeTTL time.Duration
	flag.StringVar(&address, "address", envOrDefaultValue("ADDRESS", "0.0.0.0:8080"), "")

	flag.DurationVar(&terminationGracePeriod, "termination-grace-period", envOrDefaultValue("TERMINATION_GRACE_PERIOD", 10*time.Second), "The duration the application needs to terminate gracefully")
	flag.DurationVar(&lameduck, "lameduck", envOrDefaultValue("LAMEDUCK", 1*time.Second), "A period that explicitly asks clients to stop sending requests, although the backend task is listening on that port and can provide the service")
	flag.BoolVar(&keepAlive, "http-keepalive", envOrDefaultValue("HTTP_KEEPALIVE", true), "")
	flag.IntVar(&maxConnections, "max-connections", envOrDefaultValue("MAX_CONNECTIONS", 65532), "Maximum number of connections")

	flag.StringVar(&clientID, "client-id", envOrDefaultValue("SLACK_CLIENT_ID", ""), "Slack App client ID")
	flag.StringVar(&clientSecret, "client-secret", envOrDefaultValue("SLACK_CLIENT_SECRET", ""), "Slack App client secret")
	flag.StringVar(&baseURL, "base-url", envOrDefaultValue("BASE_URL", ""), "Base URL for OAuth callback (e.g. https://slack-token-server.example.com)")
	flag.StringVar(&redisAddress, "redis-address", envOrDefaultValue("REDIS_ADDRESS", "localhost:6379"), "Redis address")
	flag.DurationVar(&deviceCodeTTL, "device-code-ttl", envOrDefaultValue("DEVICE_CODE_TTL", 10*time.Minute), "Device code TTL")
	flag.Parse()

	if clientID == "" {
		log.Fatal("--client-id or SLACK_CLIENT_ID is required")
	}
	if clientSecret == "" {
		log.Fatal("--client-secret or SLACK_CLIENT_SECRET is required")
	}
	if baseURL == "" {
		log.Fatal("--base-url or BASE_URL is required")
	}

	ctx := context.Background()

	redisClient := redis.NewClient(&redis.Options{
		Addr:     redisAddress,
		PoolSize: 10,
	})
	defer func() {
		_ = redisClient.Close()
	}()

	runtime.SetMutexProfileFraction(1)
	runtime.SetBlockProfileRate(1)

	profiler, err := pyroscope.Start(pyroscope.Config{
		ApplicationName: "slack-token-server",
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
	meter := sdkmetric.NewMeterProvider(sdkmetric.WithReader(exporter)).Meter("slack-token-server")
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

	// POST /device/authorize - Device Authorization Endpoint
	// https://www.rfc-editor.org/rfc/rfc8628#section-3.1 (Request)
	// https://www.rfc-editor.org/rfc/rfc8628#section-3.2 (Response)
	mux.HandleFuncWithMiddleware("POST /device/authorize", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
		scope := r.FormValue("scope")

		deviceCode, err := generateCode()
		if err != nil {
			slog.Error("failed to generate device code", "error", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		// Generate a separate OAuth state parameter to avoid exposing device_code in URLs
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
		pipe.Set(r.Context(), redisKey(deviceCode), string(stateJSON), deviceCodeTTL)
		pipe.Set(r.Context(), oauthStateKey(oauthState), deviceCode, deviceCodeTTL)
		if _, err := pipe.Exec(r.Context()); err != nil {
			slog.Error("failed to store device code", "error", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		// Slack uses comma-separated scopes in user_scope parameter
		slackScope := strings.ReplaceAll(scope, " ", ",")
		authURL := fmt.Sprintf("https://slack.com/oauth/v2/authorize?client_id=%s&user_scope=%s&state=%s&redirect_uri=%s",
			url.QueryEscape(clientID),
			url.QueryEscape(slackScope),
			url.QueryEscape(oauthState),
			url.QueryEscape(baseURL+"/callback"),
		)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"device_code":      deviceCode,
			"verification_uri": authURL,
			"expires_in":       int(deviceCodeTTL.Seconds()),
			"interval":         5,
		})
	})

	// GET /callback - Slack OAuth redirect handler (not part of RFC 8628; bridges Slack OAuth to device flow)
	mux.HandleFuncWithMiddleware("GET /callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		oauthState := r.URL.Query().Get("state")

		if code == "" || oauthState == "" {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		// Resolve OAuth state to device code and consume the state (one-time use)
		deviceCode, err := redisClient.GetDel(r.Context(), oauthStateKey(oauthState)).Result()
		if err != nil {
			slog.Error("failed to resolve oauth state", "error", err)
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		stored, err := redisClient.Get(r.Context(), redisKey(deviceCode)).Result()
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

		tokenResponse, err := exchangeCode(r.Context(), clientID, clientSecret, code, baseURL+"/callback")
		if err != nil {
			slog.Error("failed to exchange code", "error", err)

			// Mark device state as error so CLI gets a definitive failure instead of polling until expiry
			ds.Status = "error"
			if updatedJSON, marshalErr := json.Marshal(ds); marshalErr == nil {
				redisClient.Set(r.Context(), redisKey(deviceCode), string(updatedJSON), deviceCodeTTL)
			}

			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		// Normalize Slack response to RFC 6749 Section 5.1 token response format
		// https://www.rfc-editor.org/rfc/rfc6749#section-5.1
		normalizedToken := normalizeTokenResponse(tokenResponse, ds.Scope)
		tokenJSON, err := json.Marshal(normalizedToken)
		if err != nil {
			slog.Error("failed to marshal token response", "error", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		ds.Status = "complete"
		ds.Token = string(tokenJSON)
		updatedJSON, err := json.Marshal(ds)
		if err != nil {
			slog.Error("failed to marshal updated state", "error", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		if err := redisClient.Set(r.Context(), redisKey(deviceCode), string(updatedJSON), deviceCodeTTL).Err(); err != nil {
			slog.Error("failed to store token", "error", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Authorization successful. You can close this window."))
	})

	// POST /token - Device Access Token Request/Response
	// https://www.rfc-editor.org/rfc/rfc8628#section-3.4 (Request)
	// https://www.rfc-editor.org/rfc/rfc8628#section-3.5 (Response)
	mux.HandleFuncWithMiddleware("POST /token", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
		deviceCode := r.FormValue("device_code")
		if deviceCode == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": "invalid_request",
			})
			return
		}

		stored, err := redisClient.Get(r.Context(), redisKey(deviceCode)).Result()
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

		// https://www.rfc-editor.org/rfc/rfc8628#section-3.5 slow_down
		pollKey := redisKey(deviceCode) + ":last_poll"
		interval := ds.Interval
		if interval <= 0 {
			interval = 5
		}
		if lastPoll, err := redisClient.Get(r.Context(), pollKey).Int64(); err == nil {
			if time.Now().Unix()-lastPoll < int64(interval) {
				ds.Interval = interval + 5
				updatedJSON, err := json.Marshal(ds)
				if err == nil {
					redisClient.Set(r.Context(), redisKey(deviceCode), string(updatedJSON), deviceCodeTTL)
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
			if err := redisClient.Del(r.Context(), redisKey(deviceCode), pollKey).Err(); err != nil {
				slog.Warn("failed to delete device code", "error", err)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": "server_error",
			})
			return
		}

		// Token is ready - return it and delete from Redis
		if err := redisClient.Del(r.Context(), redisKey(deviceCode), pollKey).Err(); err != nil {
			slog.Warn("failed to delete device code", "error", err)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(ds.Token))
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

func normalizeTokenResponse(slackResponse map[string]interface{}, scope string) map[string]interface{} {
	token := map[string]interface{}{
		"token_type": "bearer",
		"scope":      scope,
	}

	// user_scope flow: token is in authed_user.access_token
	if authedUser, ok := slackResponse["authed_user"].(map[string]interface{}); ok {
		if accessToken, ok := authedUser["access_token"].(string); ok {
			token["access_token"] = accessToken
		}
		if userScope, ok := authedUser["scope"].(string); ok {
			token["scope"] = userScope
		}
	}

	// bot scope flow: token is at top level
	if _, hasUserToken := token["access_token"]; !hasUserToken {
		if accessToken, ok := slackResponse["access_token"].(string); ok {
			token["access_token"] = accessToken
		}
		if topScope, ok := slackResponse["scope"].(string); ok {
			token["scope"] = topScope
		}
	}

	return token
}

func redisKey(deviceCode string) string {
	return fmt.Sprintf("slack-token-server:device:%s", deviceCode)
}

func oauthStateKey(state string) string {
	return fmt.Sprintf("slack-token-server:oauth-state:%s", state)
}

func exchangeCode(ctx context.Context, clientID string, clientSecret string, code string, redirectURI string) (map[string]interface{}, error) {
	values := url.Values{
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"code":          {code},
		"redirect_uri":  {redirectURI},
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://slack.com/api/oauth.v2.access", strings.NewReader(values.Encode()))
	if err != nil {
		return nil, xerrors.Errorf("failed to create request: %w", err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 30 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return nil, xerrors.Errorf("failed to do request: %w", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	if response.StatusCode >= 400 {
		body, _ := io.ReadAll(response.Body)
		return nil, xerrors.Errorf("API error: status=%d, body=%s", response.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return nil, xerrors.Errorf("failed to decode response: %w", err)
	}

	ok, _ := result["ok"].(bool)
	if !ok {
		return nil, xerrors.Errorf("Slack API error: %v", result["error"])
	}

	return result, nil
}
