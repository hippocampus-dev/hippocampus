# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

hippocampus-server is a gRPC/HTTP server implementation built with Rust, featuring:
- Dual protocol support: gRPC (via Tonic) and HTTP (via Axum)
- OpenTelemetry integration for distributed tracing and metrics
- Prometheus metrics exporter
- Health check endpoints
- CPU profiling capabilities (pprof)
- Optional jemalloc allocator for improved memory management

## Common Development Commands

### Building and Testing
- `cargo build` - Build the server
- `cargo build --features jemalloc` - Build with jemalloc allocator
- `cargo test` - Run tests
- `cargo fmt` - Format code
- `cargo clippy --fix` - Lint and fix issues
- `cargo udeps --all-targets --all-features` - Check for unused dependencies

### Cross-compilation (from root directory)
- `cross build --bin hippocampus-server --release --target x86_64-unknown-linux-gnu`
- `cross build --bin hippocampus-server --release --target x86_64-unknown-linux-musl`

### Running the Server
```bash
cargo run -- --address 127.0.0.1:8080 --monitor-address 127.0.0.1:8081
```

Command-line options:
- `--address` - Main gRPC server address (default: 127.0.0.1:8080)
- `--monitor-address` - HTTP monitoring server address (default: 127.0.0.1:8081)
- `--config-file-path` - Optional config file path
- `--key-file` - Optional key file for TLS
- `--lameduck` - Graceful shutdown delay in seconds (default: 1)
- `--http-keepalive` - Enable HTTP keepalive (default: true)

## Architecture

### Server Structure
The server runs two separate services:
1. **Main gRPC Server** (port 8080): Handles application logic
   - Implements the `Greeter` service defined in `proto/helloworld.proto`
   - Includes gRPC health checking via `tonic-health`
   
2. **Monitor HTTP Server** (port 8081): Provides operational endpoints
   - `/metrics` - Prometheus metrics
   - `/debug/pprof/profile` - CPU profiling endpoint

### Key Components
- `src/main.rs` - Server initialization, OpenTelemetry setup, graceful shutdown handling
- `src/service.rs` - gRPC service implementation (Greeter)
- `src/handler.rs` - HTTP handler implementations
- `src/handler/metrics.rs` - Prometheus metrics endpoint
- `src/handler/debug/pprof.rs` - CPU profiling support
- `src/middleware.rs` - Middleware implementations:
  - HTTP middleware (trace propagation via `propagator`)
  - gRPC tower layer (trace propagation via `TonicTracingLayer` and `TonicTracingService`)
- `build.rs` - Protobuf compilation via tonic-build

### Middleware Architecture
The gRPC server uses tower layers for cross-cutting concerns:
- **TonicTracingLayer**: Custom tower layer that wraps gRPC services for distributed tracing
- **TonicTracingService**: Service implementation that extracts W3C Trace Context from HTTP headers and creates spans with OpenTelemetry semantic conventions
- Integration with OpenTelemetry for automatic trace propagation across service boundaries
- Applied via `Server::builder().layer()` following the pattern from [tonic PR #651](https://github.com/hyperium/tonic/pull/651)

### Protocol Buffer Integration
The server uses protobuf definitions from `/opt/hippocampus/proto/`:
- `helloworld.proto` - Defines the Greeter service
- Google API annotations are supported for HTTP/gRPC transcoding

### Observability
- **Tracing**: OpenTelemetry distributed tracing with OTLP exporter
- **Metrics**: OpenTelemetry metrics exposed via Prometheus format
- **Propagation**: W3C Trace Context propagation between services
  - HTTP services: via `middleware::propagator` using Axum middleware
  - gRPC services: via `middleware::TonicTracingLayer` using tower layers
- **Structured Logging**: JSON logs in production, human-readable in development

### Feature Flags
- `tracing` (default): Enables OpenTelemetry tracing
- `jemalloc`: Uses jemalloc instead of system allocator for better performance

### Graceful Shutdown
The server implements graceful shutdown on SIGTERM:
1. Receives SIGTERM signal
2. Waits for `lameduck` period (default 1 second)
3. Stops accepting new connections
4. Completes existing requests
5. Shuts down cleanly