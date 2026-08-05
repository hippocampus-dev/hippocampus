package myhttp

import (
	"log/slog"
	"net/http"

	"go.opentelemetry.io/otel/metric"
)

func newServerMux(logger *slog.Logger, httpRequestsDurationMicroSeconds metric.Int64Histogram) *myRouter {
	return &myRouter{
		ServeMux:                         http.NewServeMux(),
		logger:                           logger,
		httpRequestsDurationMicroSeconds: httpRequestsDurationMicroSeconds,
	}
}

var NewServerMux = newServerMux
