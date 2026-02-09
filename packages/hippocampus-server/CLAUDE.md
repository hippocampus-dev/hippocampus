# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

hippocampus-server is a gRPC/HTTP server implementation built with Rust, featuring:
- Full-text search capabilities via hippocampus-core integration
- Dual protocol support: gRPC (via Tonic) and HTTP (via Axum)
- OpenTelemetry integration for distributed tracing and metrics
- Prometheus metrics exporter
- Health check endpoints
- CPU profiling capabilities (pprof)
- Optional jemalloc allocator for improved memory management
- TOML configuration file support for storage, tokenizer, and schema settings
- OpenAPI spec generation from protobuf definitions

## Common Development Commands

### Building and Testing
- `cargo build` - Build the server
- `cargo build --features jemalloc` - Build with jemalloc allocator
- `cargo test` - Run tests
- `cargo fmt` - Format code
- `cargo clippy --fix` - Lint and fix issues
- `cargo udeps --all-targets --all-features` - Check for unused dependencies

### OpenAPI Documentation
- `make openapi` - Generate OpenAPI spec from protobuf definitions (run from repository root, outputs to `packages/hippocampus-server/openapi/api.swagger.json`)

### Cross-compilation (from root directory)
- `cross build --bin hippocampus-server --release --target x86_64-unknown-linux-gnu`
- `cross build --bin hippocampus-server --release --target x86_64-unknown-linux-musl`

### Running the Server
```bash
# Basic server (HelloWorld only)
cargo run -- --address 127.0.0.1:8080 --monitor-address 127.0.0.1:8081

# With hippocampus-core search capabilities
cargo run -- --config-file-path config.toml --address 127.0.0.1:8080 --monitor-address 127.0.0.1:8081
```

Command-line options:
- `--address` - Main gRPC server address (default: 127.0.0.1:8080)
- `--monitor-address` - HTTP monitoring server address (default: 127.0.0.1:8081)
- `--config-file-path` - Config file path (enables hippocampus search service)
- `--key-file` - Optional key file for TLS
- `--lameduck` - Graceful shutdown delay in seconds (default: 1)

### Configuration File
When `--config-file-path` is provided, the server enables the Hippocampus search service. See `config.example.toml` for a complete example.

The storage configuration supports hybrid setups where document storage and token storage can use different backends. The format is compatible with `hippocampus-standalone`:

```toml
[TokenStorage]
kind = "File"
path = "/var/hippocampus/tokens"

# GCS example (requires gcs feature):
# [TokenStorage]
# kind = "GCS"
# bucket = "my-bucket"
# prefix = "tokens"
# service_account_key_path = "/path/to/key.json"

[DocumentStorage]
kind = "File"
path = "/var/hippocampus/documents"

[Tokenizer]
kind = "Lindera"

[Schema]

[[Schema.fields]]
name = "content"
type = "string"
indexed = true
```

## Architecture

### Server Structure
The server runs two separate services:
1. **Main gRPC Server** (port 8080): Handles application logic
   - Implements the `Greeter` service defined in `proto/helloworld.proto`
   - Implements the `Hippocampus` service defined in `proto/hippocampus.proto` (when config file is provided)
   - Includes gRPC health checking via `tonic-health`

2. **Monitor HTTP Server** (port 8081): Provides operational endpoints
   - `/metrics` - Prometheus metrics
   - `/debug/pprof/profile` - CPU profiling endpoint
   - `/openapi.json` - OpenAPI specification

### Hippocampus gRPC Service
When started with a configuration file, the server provides full-text search capabilities:
- `Index(IndexRequest)` - Index a document with field values
- `Search(SearchRequest)` - Search indexed documents with a query string

### Key Components
- `src/main.rs` - Server initialization, OpenTelemetry setup, graceful shutdown handling
- `src/service.rs` - gRPC service implementations (Greeter, HippocampusService)
- Configuration parsing via `hippocampus-configuration` crate (shared with `hippocampus-standalone`)
- `src/handler.rs` - HTTP handler implementations
- `src/handler/metrics.rs` - Prometheus metrics endpoint
- `src/handler/debug/pprof.rs` - CPU profiling support
- `src/handler/openapi.rs` - OpenAPI spec endpoint
- `src/middleware.rs` - Middleware implementations:
  - gRPC tower layer (trace propagation via `TracingLayer` and `TracingService`)
- `build.rs` - Protobuf compilation via tonic-build
- `config.example.toml` - Example configuration file

### Middleware Architecture
The gRPC server uses tower layers for cross-cutting concerns:
- **TonicTracingLayer**: Custom tower layer that wraps gRPC services for distributed tracing
- **TonicTracingService**: Service implementation that extracts W3C Trace Context from HTTP headers and creates spans with OpenTelemetry semantic conventions
- Integration with OpenTelemetry for automatic trace propagation across service boundaries
- Applied via `Server::builder().layer()` following the pattern from [tonic PR #651](https://github.com/hyperium/tonic/pull/651)

### Protocol Buffer Integration
The server uses protobuf definitions from `/opt/hippocampus/proto/`:
- `helloworld.proto` - Defines the Greeter service
- `hippocampus.proto` - Defines the Hippocampus search service (Index, Search)
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
- `wasm`: Enables WASM tokenizer plugin support (requires `[tokenizer.wasm]` configuration)
- `sqlite`: Enables SQLite storage backend for both document and token storage
- `cassandra`: Enables Cassandra token storage backend (token storage only)

### Graceful Shutdown
The server implements graceful shutdown on SIGTERM:
1. Receives SIGTERM signal
2. Waits for `lameduck` period (default 1 second)
3. Stops accepting new connections
4. Completes existing requests
5. Shuts down cleanly
