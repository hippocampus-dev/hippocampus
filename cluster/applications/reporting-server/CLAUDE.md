# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

The reporting-server is a Go-based HTTP service that implements the W3C Reporting API to collect browser-generated security and error reports. It handles CSP violations, network errors, deprecation warnings, crash reports, and various policy violations. The service integrates with the Hippocampus platform's observability stack, providing metrics, tracing, and continuous profiling.

## Common Development Commands

### Development Workflow
- `make dev` - Run server with hot-reload using watchexec (automatically restarts on file changes)
- `go run *.go` - Run server directly
- `go build` - Build the binary
- `go mod tidy` - Clean up dependencies

### Docker Operations
- `docker build -t reporting-server .` - Build Docker image
- The image uses distroless/static:nonroot for security and runs as UID 65532

## High-Level Architecture

### Core Components

1. **Report Handlers**
   - `/csp-reports` - Legacy CSP violation endpoint (application/csp-report)
   - `/reports` - Modern Reporting API endpoint (application/reports+json)
   - Supports 7 report types: CSP, Network Error, Deprecation, Crash, Intervention, Permissions Policy, Document Policy

2. **Middleware Stack**
   - Custom `myRouter` wrapper providing:
     - OpenTelemetry tracing per request
     - Panic recovery with logging
     - Request duration metrics
     - Pyroscope profiling labels
     - Structured logging with trace correlation

3. **Observability Integration**
   - **Metrics**: Prometheus endpoint at `/metrics` with counters for each report type
   - **Tracing**: OpenTelemetry OTLP export to collector
   - **Profiling**: Continuous profiling with Pyroscope (CPU, memory, goroutines, mutex, block)
   - **Logging**: JSON structured logs with trace/span IDs

### Key Design Patterns

1. **Graceful Shutdown**: Configurable termination period with lameduck mode
2. **CORS Support**: Allows cross-origin report submission
3. **Debug Mode**: Exposes pprof endpoints when DEBUG=true
4. **Environment Configuration**: Uses godotenv for .env file support

### Configuration Options

Command-line flags:
- `--address` - Listen address (default: 0.0.0.0:8080)
- `--termination-grace-period-seconds` - Graceful shutdown duration (default: 10s)
- `--lameduck` - Period to reject new requests before shutdown (default: 1s)
- `--http-keepalive` - Enable HTTP keep-alive (default: true)
- `--max-connections` - Maximum concurrent connections (default: 65532)

Environment variables:
- `OTEL_EXPORTER_OTLP_ENDPOINT` - OpenTelemetry collector endpoint
- `PYROSCOPE_ENDPOINT` - Pyroscope server endpoint
- `DEBUG` - Enable debug mode with pprof endpoints

### Integration Notes

- Follows Hippocampus microservice patterns with container-first approach
- Designed for Kubernetes deployment with health checks at `/healthz`
- No test files currently exist - testing strategy would need implementation
- All report structures match W3C Reporting API specifications