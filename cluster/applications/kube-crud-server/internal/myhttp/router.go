package myhttp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"slices"
	"time"

	"github.com/grafana/pyroscope-go"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"k8s.io/client-go/dynamic"
)

type Middleware func(http.Handler) http.Handler

type myRouter struct {
	*http.ServeMux
	logger                           *slog.Logger
	httpRequestsDurationMicroSeconds metric.Int64Histogram
	dynamicClient                    *dynamic.DynamicClient
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
