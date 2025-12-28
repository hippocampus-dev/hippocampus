package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"kube-crud-server/internal/myhttp"
	"kube-crud-server/internal/routes"
	"log"
	"log/slog"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"sync"
	"syscall"
	"time"

	otelpyroscope "github.com/grafana/otel-profiling-go"
	"github.com/grafana/pyroscope-go"
	pyroscopepprof "github.com/grafana/pyroscope-go/http/pprof"
	"github.com/joho/godotenv"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	otelprometheus "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/net/netutil"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
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

var Debug = false

type auditLogEntry struct {
	Time           string      `json:"time"`
	Host           string      `json:"host"`
	Method         string      `json:"method"`
	Uri            string      `json:"uri"`
	Protocol       string      `json:"protocol"`
	Reqtime        int64       `json:"reqtime"`
	RequestHeaders http.Header `json:"request_headers"`
	RequestBody    string      `json:"request_body,omitempty"`
	TraceID        string      `json:"trace_id"`
	SpanID         string      `json:"span_id"`
}

func newAuditLogger(filePath string) (func(http.Handler) http.Handler, error) {
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, err
	}

	var mutex sync.Mutex

	middleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startTime := time.Now()

			requestBody, err := io.ReadAll(r.Body)
			if err != nil {
				requestBody = []byte{}
			}
			r.Body = io.NopCloser(bytes.NewBuffer(requestBody))

			span := trace.SpanFromContext(r.Context())

			next.ServeHTTP(w, r)

			entry := auditLogEntry{
				Time:           startTime.Format(time.RFC3339Nano),
				Host:           r.RemoteAddr,
				Reqtime:        time.Since(startTime).Milliseconds(),
				Method:         r.Method,
				Uri:            r.URL.Path,
				Protocol:       r.Proto,
				RequestHeaders: r.Header,
				RequestBody:    string(requestBody),
				TraceID:        span.SpanContext().TraceID().String(),
				SpanID:         span.SpanContext().SpanID().String(),
			}

			mutex.Lock()
			defer mutex.Unlock()

			_ = json.NewEncoder(file).Encode(entry)
		})
	}

	return middleware, nil
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
	var auditLogPath string
	flag.StringVar(&address, "address", envOrDefaultValue("ADDRESS", "0.0.0.0:8080"), "")
	flag.StringVar(&auditLogPath, "audit-log-path", envOrDefaultValue("AUDIT_LOG_PATH", ""), "Path to audit log file")

	flag.DurationVar(&terminationGracePeriod, "termination-grace-period", envOrDefaultValue("TERMINATION_GRACE_PERIOD", 10*time.Second), "The duration the application needs to terminate gracefully")
	flag.DurationVar(&lameduck, "lameduck", envOrDefaultValue("LAMEDUCK", 1*time.Second), "A period that explicitly asks clients to stop sending requests, although the backend task is listening on that port and can provide the service")
	flag.BoolVar(&keepAlive, "http-keepalive", envOrDefaultValue("HTTP_KEEPALIVE", true), "")
	flag.IntVar(&maxConnections, "max-connections", envOrDefaultValue("MAX_CONNECTIONS", 65532), "Maximum number of connections")
	flag.Parse()

	ctx := context.Background()

	runtime.SetMutexProfileFraction(1)
	runtime.SetBlockProfileRate(1)

	profiler, err := pyroscope.Start(pyroscope.Config{
		ApplicationName: "kube-crud-server",
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
	meter := sdkmetric.NewMeterProvider(sdkmetric.WithReader(exporter)).Meter("kube-crud-server")
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

	kubeConfig, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("failed to create kubernetes config: %+v", err)
	}
	clientset, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		log.Fatalf("failed to create kubernetes dynamic client: %+v", err)
	}
	dynamicClient, err := dynamic.NewForConfig(kubeConfig)
	if err != nil {
		log.Fatalf("failed to create kubernetes dynamic client: %+v", err)
	}

	mux := myhttp.NewServerMux(logger, httpRequestsDurationMicroSeconds)

	if auditLogPath != "" {
		auditLogMiddleware, err := newAuditLogger(auditLogPath)
		if err != nil {
			log.Fatalf("failed to create audit logger: %+v", err)
		}
		mux.Use(auditLogMiddleware)
	}

	mux.HandleFuncWithMiddleware("POST /{namespace}/{group}/{version}/{kind}", routes.Create(dynamicClient))
	mux.HandleFuncWithMiddleware("GET /{namespace}/{group}/{version}/{kind}/{name}", routes.Read(dynamicClient))
	mux.HandleFuncWithMiddleware("PATCH /{namespace}/{group}/{version}/{kind}/{name}", routes.Update(dynamicClient))
	mux.HandleFuncWithMiddleware("DELETE /{namespace}/{group}/{version}/{kind}/{name}", routes.Delete(dynamicClient))

	mux.HandleFuncWithMiddleware("GET /{$}", routes.ListNamespaces(clientset))
	mux.HandleFuncWithMiddleware("GET /{namespace}/{group}/{version}/{kind}", routes.ListResources(dynamicClient))

	mux.HandleFuncWithMiddleware("POST /{namespace}/batch/v1/job/{name}/from/cronjob/{from}", routes.CreateBatchV1JobFromCronJob(clientset))

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
