package main

import (
	"context"
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
	"reflect"
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
)

// SecurityPolicyViolationEvent https://w3c.github.io/webappsec-csp/#securitypolicyviolationevent
type SecurityPolicyViolationEvent struct {
	// https://w3c.github.io/webappsec-csp/#idl-index
	CSPReport struct {
		DocumentURI        string  `json:"document-uri"`
		Referrer           *string `json:"referrer,omitempty"`
		BlockedURI         *string `json:"blocked-uri,omitempty"`
		EffectiveDirective string  `json:"effective-directive"`
		// historical alias of effectiveDirective
		// ViolatedDirective string `json:"violated-directive"`
		OriginalPolicy string  `json:"original-policy"`
		SourceFile     *string `json:"source-file,omitempty"`
		Sample         *string `json:"sample,omitempty"`
		Disposition    string  `json:"disposition"`
		StatusCode     uint16  `json:"status-code,omitempty"`
		LineNumber     *uint32 `json:"line-number,omitempty"`
		ColumnNumber   *uint32 `json:"column-number,omitempty"`
	} `json:"csp-report"`
}

type ReportBody struct {
	Age       uint            `json:"age"`
	Body      json.RawMessage `json:"body"`
	Type      string          `json:"type"`
	URL       string          `json:"url"`
	UserAgent string          `json:"user_agent"`
}

type ReportBodies = []ReportBody

// CSPReport https://w3c.github.io/webappsec-csp/#cspviolationreportbody
type CSPReport struct {
	DocumentURL        string  `json:"documentURL"`
	Referrer           *string `json:"referrer,omitempty"`
	BlockedURL         *string `json:"blockedURL,omitempty"`
	EffectiveDirective string  `json:"effectiveDirective"`
	OriginalPolicy     string  `json:"originalPolicy"`
	SourceFile         *string `json:"sourceFile,omitempty"`
	Sample             *string `json:"sample,omitempty"`
	Disposition        string  `json:"disposition"`
	StatusCode         uint16  `json:"statusCode,omitempty"`
	LineNumber         *uint32 `json:"lineNumber,omitempty"`
	ColumnNumber       *uint32 `json:"columnNumber,omitempty"`
}

// NetworkError https://w3c.github.io/network-error-logging/#generate-a-network-error-report
type NetworkError struct {
	SamplingFraction float64           `json:"sampling_fraction"`
	Referer          *string           `json:"referer,omitempty"`
	ServerIP         *string           `json:"server_ip,omitempty"`
	Protocol         *string           `json:"protocol,omitempty"`
	Method           *string           `json:"method,omitempty"`
	RequestHeaders   map[string]string `json:"request_headers,omitempty"`
	ResponseHeaders  map[string]string `json:"response_headers,omitempty"`
	StatusCode       *uint16           `json:"status_code,omitempty"`
	ElapsedTime      uint              `json:"elapsed_time"`
	Phase            string            `json:"phase"`
	Type             string            `json:"type"`
}

// DeprecationReport https://wicg.github.io/deprecation-reporting/#deprecationreportbody
type DeprecationReport struct {
	ID                 string  `json:"id"`
	AnticipatedRemoval *string `json:"anticipatedRemoval,omitempty"`
	Message            string  `json:"message"`
	SourceFile         *string `json:"sourceFile,omitempty"`
	LineNumber         *uint32 `json:"lineNumber,omitempty"`
	ColumnNumber       *uint32 `json:"columnNumber,omitempty"`
}

// CrashReport https://wicg.github.io/crash-reporting/#crash-report
type CrashReport struct {
	Reason string `json:"reason"`
}

// InterventionReport https://wicg.github.io/intervention-reporting/#intervention-report
type InterventionReport struct {
	ID           string  `json:"id"`
	Message      string  `json:"message"`
	SourceFile   *string `json:"sourceFile,omitempty"`
	LineNumber   *uint32 `json:"lineNumber,omitempty"`
	ColumnNumber *uint32 `json:"columnNumber,omitempty"`
}

// PermissionsPolicyViolationReport https://www.w3.org/TR/permissions-policy/#reporting
type PermissionsPolicyViolationReport struct {
	FeatureID    string  `json:"featureId"`
	SourceFile   *string `json:"sourceFile,omitempty"`
	LineNumber   *uint32 `json:"lineNumber,omitempty"`
	ColumnNumber *uint32 `json:"columnNumber,omitempty"`
	Disposition  string  `json:"disposition"`
}

// DocumentPolicyViolationReport https://wicg.github.io/document-policy/#algo-is-value-compatible-or-report
type DocumentPolicyViolationReport struct {
	FeatureID    string  `json:"featureId"`
	SourceFile   *string `json:"sourceFile,omitempty"`
	LineNumber   *uint32 `json:"lineNumber,omitempty"`
	ColumnNumber *uint32 `json:"columnNumber,omitempty"`
	Disposition  string  `json:"disposition"`
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
	if Debug {
		_ = godotenv.Load()
	}

	var address string
	var terminationGracePeriod time.Duration
	var lameduck time.Duration
	var keepAlive bool
	var maxConnections int
	flag.StringVar(&address, "address", envOrDefaultValue("ADDRESS", "0.0.0.0:8080"), "")

	flag.DurationVar(&terminationGracePeriod, "termination-grace-period", envOrDefaultValue("TERMINATION_GRACE_PERIOD", 10*time.Second), "The duration the application needs to terminate gracefully")
	flag.DurationVar(&lameduck, "lameduck", envOrDefaultValue("LAMEDUCK", 1*time.Second), "A period that explicitly asks clients to stop sending requests, although the backend task is listening on that port and can provide the service")
	flag.BoolVar(&keepAlive, "http-keepalive", envOrDefaultValue("HTTP_KEEPALIVE", true), "")
	flag.IntVar(&maxConnections, "max-connections", envOrDefaultValue("MAX_CONNECTIONS", 65532), "Maximum number of connections")
	flag.Parse()

	ctx := context.Background()

	runtime.SetMutexProfileFraction(1)
	runtime.SetBlockProfileRate(1)

	profiler, err := pyroscope.Start(pyroscope.Config{
		ApplicationName: "reporting-server",
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
	meter := sdkmetric.NewMeterProvider(sdkmetric.WithReader(exporter)).Meter("reporting-server")
	cspViolationsTotal, err := meter.Int64Counter("csp_violations_total")
	if err != nil {
		log.Fatalf("failed to create counter: %+v", err)
	}
	networkErrorsTotal, err := meter.Int64Counter("network_errors_total")
	if err != nil {
		log.Fatalf("failed to create counter: %+v", err)
	}
	deprecationsTotal, err := meter.Int64Counter("deprecations_total")
	if err != nil {
		log.Fatalf("failed to create counter: %+v", err)
	}
	crashesTotal, err := meter.Int64Counter("crashes_total")
	if err != nil {
		log.Fatalf("failed to create counter: %+v", err)
	}
	interventionsTotal, err := meter.Int64Counter("interventions_total")
	if err != nil {
		log.Fatalf("failed to create counter: %+v", err)
	}
	permissionsPolicyViolationsTotal, err := meter.Int64Counter("permissions_policy_violations_total")
	if err != nil {
		log.Fatalf("failed to create counter: %+v", err)
	}
	documentPolicyViolationsTotal, err := meter.Int64Counter("document_policy_violations_total")
	if err != nil {
		log.Fatalf("failed to create counter: %+v", err)
	}
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
	mux.HandleFuncWithMiddleware("/csp-reports", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		// https://github.com/w3c/reporting/issues/41
		case http.MethodOptions:
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "POST")
			w.Header().Set("Access-Control-Max-Age", "86400")
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusNoContent)
			_, _ = w.Write([]byte(http.StatusText(http.StatusNoContent)))
		case http.MethodPost:
			switch r.Header.Get("Content-Type") {
			case "application/csp-report":
				var event SecurityPolicyViolationEvent

				if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
					http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
					return
				}

				cspViolationsTotal.Add(r.Context(), 1, metric.WithAttributes(
					attribute.Key("origin").String(r.Header.Get("Origin")),
					attribute.Key("effective-directive").String(event.CSPReport.EffectiveDirective),
					attribute.Key("disposition").String(event.CSPReport.Disposition),
				))

				slog.Info("csp-violation", keyvalues(event.CSPReport)...)

				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(http.StatusText(http.StatusOK)))
			default:
				http.Error(w, http.StatusText(http.StatusUnsupportedMediaType), http.StatusUnsupportedMediaType)
			}
		default:
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFuncWithMiddleware("/reports", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		// https://github.com/w3c/reporting/issues/41
		case http.MethodOptions:
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "POST")
			w.Header().Set("Access-Control-Max-Age", "86400")
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusNoContent)
			_, _ = w.Write([]byte(http.StatusText(http.StatusNoContent)))
		case http.MethodPost:
			switch r.Header.Get("Content-Type") {
			case "application/reports+json":
				var reports ReportBodies

				if err := json.NewDecoder(r.Body).Decode(&reports); err != nil {
					http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
					return
				}

				for _, report := range reports {
					switch report.Type {
					case "csp-violation":
						var cspReport CSPReport

						if err := json.Unmarshal(report.Body, &cspReport); err != nil {
							http.Error(w, "invalid csp-violation report", http.StatusBadRequest)
							return
						}

						cspViolationsTotal.Add(r.Context(), 1, metric.WithAttributes(
							attribute.Key("effective-directive").String(cspReport.EffectiveDirective),
							attribute.Key("disposition").String(cspReport.Disposition),
						))

						slog.Info(report.Type, keyvalues(cspReport)...)
					case "network-error":
						var networkError NetworkError

						if err := json.Unmarshal(report.Body, &networkError); err != nil {
							http.Error(w, "invalid network-error report", http.StatusBadRequest)
							return
						}

						networkErrorsTotal.Add(r.Context(), 1, metric.WithAttributes(
							attribute.Key("phase").String(networkError.Phase),
							attribute.Key("type").String(networkError.Type),
						))

						slog.Info(report.Type, keyvalues(networkError)...)
					case "deprecation":
						var deprecationReport DeprecationReport

						if err := json.Unmarshal(report.Body, &deprecationReport); err != nil {
							http.Error(w, "invalid deprecation report", http.StatusBadRequest)
							return
						}

						deprecationsTotal.Add(r.Context(), 1, metric.WithAttributes(
							attribute.Key("id").String(deprecationReport.ID),
						))

						slog.Info(report.Type, keyvalues(deprecationReport)...)
					case "crash":
						var crashReport CrashReport

						if err := json.Unmarshal(report.Body, &crashReport); err != nil {
							http.Error(w, "invalid crash report", http.StatusBadRequest)
							return
						}

						crashesTotal.Add(r.Context(), 1, metric.WithAttributes(
							attribute.Key("reason").String(crashReport.Reason),
						))

						slog.Info(report.Type, keyvalues(crashReport)...)
					case "intervention":
						var interventionReport InterventionReport

						if err := json.Unmarshal(report.Body, &interventionReport); err != nil {
							http.Error(w, "invalid intervention report", http.StatusBadRequest)
							return
						}

						interventionsTotal.Add(r.Context(), 1, metric.WithAttributes(
							attribute.Key("id").String(interventionReport.ID),
						))

						slog.Info(report.Type, keyvalues(interventionReport)...)
					case "permissions-policy-violation":
						var permissionsPolicyViolationReport PermissionsPolicyViolationReport

						if err := json.Unmarshal(report.Body, &permissionsPolicyViolationReport); err != nil {
							http.Error(w, "invalid permissions-policy-violation report", http.StatusBadRequest)
							return
						}

						permissionsPolicyViolationsTotal.Add(r.Context(), 1, metric.WithAttributes(
							attribute.Key("featureId").String(permissionsPolicyViolationReport.FeatureID),
							attribute.Key("disposition").String(permissionsPolicyViolationReport.Disposition),
						))

						slog.Info(report.Type, keyvalues(permissionsPolicyViolationReport)...)
					case "document-policy-violation":
						var documentPolicyViolationReport DocumentPolicyViolationReport

						if err := json.Unmarshal(report.Body, &documentPolicyViolationReport); err != nil {
							http.Error(w, "invalid document-policy-violation report", http.StatusBadRequest)
							return
						}

						documentPolicyViolationsTotal.Add(r.Context(), 1, metric.WithAttributes(
							attribute.Key("featureId").String(documentPolicyViolationReport.FeatureID),
							attribute.Key("disposition").String(documentPolicyViolationReport.Disposition),
						))

						slog.Info(report.Type, keyvalues(documentPolicyViolationReport)...)
					default:
						slog.Error("unknown report type", "type", report.Type)
					}
				}

				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(http.StatusText(http.StatusOK)))
			default:
				http.Error(w, http.StatusText(http.StatusUnsupportedMediaType), http.StatusUnsupportedMediaType)
			}
		default:
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		}
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

func keyvalues(i any) []any {
	var keyvalues []any

	v := reflect.ValueOf(i)
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		key := t.Field(i)
		jsonTag := key.Tag.Get("json")
		switch jsonTag {
		case "-":
			continue
		case "":
			keyvalues = append(keyvalues, key.Name)
		default:
			name, _, _ := strings.Cut(jsonTag, ",")
			keyvalues = append(keyvalues, name)
		}

		value := v.Field(i)
		if value.Kind() == reflect.Pointer && !value.IsNil() {
			keyvalues = append(keyvalues, value.Elem().Interface())
		} else {
			keyvalues = append(keyvalues, value.Interface())
		}
	}

	return keyvalues
}
