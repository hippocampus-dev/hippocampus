# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This project provides WebAssembly (WASM) filters for Envoy proxy using the proxy-wasm SDK. These filters extend Envoy's HTTP request/response processing capabilities.

The workspace contains 8 independent WASM filters:
- `cookie-manipulator` - Manages HTTP cookies
- `envoy-trusted-header` - Validates headers based on SPIFFE certificates
- `envoy-xauthrequest` - External authentication request handling
- `fallback-filter` - Provides fallback responses on specific HTTP status codes
- `header-getter/setter` - Manipulates HTTP headers
- `metrics/metrics-exporter` - Collects and exports metrics

## Common Development Commands

### Primary Development
- `make dev` - Development with hot-reload (requires Kubernetes cluster)
- `make all` - Format, lint, test, and build all targets
- `make fmt` - Format Rust code with cargo fmt
- `make lint` - Lint with cargo clippy --fix
- `make test` - Run tests with cross for Linux target
- `make targets` - Build for x86_64-unknown-linux-gnu

### WASM-specific Build
- `cargo build --target=wasm32-unknown-unknown --release` - Build WASM artifacts
- Individual package build: `cargo build -p <package-name> --target=wasm32-unknown-unknown --release`

### Testing
- `make e2e` - Creates Kind cluster, deploys filters, runs k6 tests
- Run specific k6 test: `k6 run k6/<filter-name>/index.js`

### Kubernetes Development
- `skaffold dev --port-forward` - Continuous development with auto-rebuild/deploy
- `skaffold run` - One-time deployment

## Architecture

### Filter Structure
All filters follow the proxy-wasm SDK pattern:
1. `_start` entry point initializes the root context
2. `RootContext` handles filter configuration
3. `HttpContext` processes individual HTTP requests/responses

### Configuration
Each filter accepts JSON configuration via Envoy's WASM plugin configuration:
```rust
#[derive(Deserialize)]
struct FilterConfig {
    log_level: Option<LogLevel>,
    // Filter-specific fields
}
```

### Testing Approach
- Unit tests: Standard Rust tests in each package
- Integration tests: K6 scripts in `k6/` directory test filter behavior through HTTP requests
- E2E tests create an isolated Kind cluster with Envoy and the filters deployed

### Deployment
Filters are built into WASM artifacts and served via HTTP (nghttpd on port 8080) or loaded from filesystem. Envoy loads them as HTTP filters in its filter chain configuration.