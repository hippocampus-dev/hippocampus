package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"image"
	_ "image/jpeg"
	"image/png"
	"io"
	"log"
	"log/slog"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	diffimage "snapshot-controller/internal/diff/image"
	difftext "snapshot-controller/internal/diff/text"
	"snapshot-controller/internal/myhttp"
	"strconv"
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
	"golang.org/x/net/netutil"
	"golang.org/x/xerrors"
)

type Server struct {
	address                string
	terminationGracePeriod time.Duration
	lameduck               time.Duration
	keepAlive              bool
	maxConnections         int
}

func NewServer() *Server {
	return &Server{
		address:                envOrDefaultValue("ADDRESS", "0.0.0.0:8080"),
		terminationGracePeriod: envOrDefaultValue("TERMINATION_GRACE_PERIOD", 10*time.Second),
		lameduck:               envOrDefaultValue("LAMEDUCK", 1*time.Second),
		keepAlive:              envOrDefaultValue("HTTP_KEEPALIVE", true),
		maxConnections:         envOrDefaultValue("MAX_CONNECTIONS", 65532),
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

var Debug = false

func (s *Server) Start(ctx context.Context) error {
	runtime.SetMutexProfileFraction(1)
	runtime.SetBlockProfileRate(1)

	profiler, err := pyroscope.Start(pyroscope.Config{
		ApplicationName: "diff-server",
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

	r, err := sdkresource.Merge(
		sdkresource.Default(),
		sdkresource.NewWithAttributes(semconv.SchemaURL),
	)
	if err != nil {
		return xerrors.Errorf("failed to create resource: %w", err)
	}
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
	// NOTE: Gauge(UpDownCounter), Summary or Untyped does not support exemplars
	// https://github.com/prometheus/client_golang/blob/v1.20.4/prometheus/metric.go#L200
	meter := sdkmetric.NewMeterProvider(sdkmetric.WithReader(exporter)).Meter("diff-server")
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

	mux := myhttp.NewServerMux(logger, httpRequestsDurationMicroSeconds)

	mux.HandleFuncWithMiddleware("POST /diff", s.handleDiff)

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

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM)
	<-quit
	time.Sleep(s.lameduck)

	ctx, cancel := context.WithTimeout(ctx, s.terminationGracePeriod)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		return xerrors.Errorf("failed to shutdown server: %w", err)
	}

	if err := traceProvider.Shutdown(ctx); err != nil {
		return xerrors.Errorf("failed to shutdown trace provider: %w", err)
	}

	if err := profiler.Stop(); err != nil {
		return xerrors.Errorf("failed to shutdown profiler: %w", err)
	}

	return nil
}

type DiffResponse struct {
	DiffData   string  `json:"diffData"`
	DiffAmount float64 `json:"diffAmount"`
}

func (s *Server) handleDiff(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	format := r.FormValue("format")
	if format == "" {
		format = "pixel"
	}

	baselineFile, _, err := r.FormFile("baseline")
	if err != nil {
		http.Error(w, "missing baseline file", http.StatusBadRequest)
		return
	}
	defer baselineFile.Close()

	targetFile, _, err := r.FormFile("target")
	if err != nil {
		http.Error(w, "missing target file", http.StatusBadRequest)
		return
	}
	defer targetFile.Close()

	baselineData, err := io.ReadAll(baselineFile)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	targetData, err := io.ReadAll(targetFile)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	switch format {
	case "pixel", "rectangle":
		baselineImage, err := decodeImage(baselineData)
		if err != nil {
			http.Error(w, "invalid baseline image", http.StatusBadRequest)
			return
		}

		targetImage, err := decodeImage(targetData)
		if err != nil {
			http.Error(w, "invalid target image", http.StatusBadRequest)
			return
		}

		var diffResult *diffimage.DiffResult

		if format == "pixel" {
			diffResult = diffimage.NewPixelDiff(0.1).Calculate(baselineImage, targetImage)
		} else {
			diffResult = diffimage.NewRectangleDiff().Calculate(baselineImage, targetImage)
		}

		var buffer bytes.Buffer
		if err := png.Encode(&buffer, diffResult.Image); err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(DiffResponse{
			DiffData:   base64.StdEncoding.EncodeToString(buffer.Bytes()),
			DiffAmount: diffResult.DiffAmount,
		}); err != nil {
			slog.Error("failed to encode response", "error", err)
		}
		return

	case "line", "dom":
		var diffResult *difftext.DiffResult
		var err error

		if format == "line" {
			diffResult, err = difftext.NewLineDiff().Calculate(baselineData, targetData)
		} else {
			diffResult, err = difftext.NewDOMDiff().Calculate(baselineData, targetData)
		}

		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(DiffResponse{
			DiffData:   base64.StdEncoding.EncodeToString(diffResult.Diff),
			DiffAmount: diffResult.DiffAmount,
		}); err != nil {
			slog.Error("failed to encode response", "error", err)
		}
	default:
		http.Error(w, "unsupported format", http.StatusBadRequest)
		return
	}
}

func decodeImage(data []byte) (image.Image, error) {
	i, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return i, nil
}

func main() {
	if Debug {
		_ = godotenv.Load()
	}

	ctx := context.Background()

	server := NewServer()
	if err := server.Start(ctx); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
