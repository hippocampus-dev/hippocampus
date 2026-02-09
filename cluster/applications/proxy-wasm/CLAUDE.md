# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This project provides WebAssembly (WASM) filters for Envoy proxy using the proxy-wasm SDK. These filters extend Envoy's HTTP request/response processing capabilities.

The workspace contains 9 packages (8 WASM filters + 1 shared library):
- `cookie-manipulator` - Manages HTTP cookies
- `envoy-trusted-header` - Validates headers based on SPIFFE certificates
- `envoy-xauthrequest` - External authentication request handling
- `fallback-filter` - Provides fallback responses on specific HTTP status codes
- `finalizer` - Cleans up shared data on request completion
- `header-getter` - Extracts header values and stores in shared data
- `header-setter` - Sets headers from shared data values
- `metrics` - Shared library for metric type definitions (not a filter)
- `metrics-exporter` - Exports metrics in Prometheus format

Additionally, there is a standalone gRPC service (excluded from workspace):
- `ext-proc/` - Envoy External Processor that computes SHA256 hashes and adds request headers (proxy-wasm cannot modify request headers from body phase, so ext_proc is required for body hash)

### ext-proc Hash Headers

| Header | Source | CLI Option | Default |
|--------|--------|------------|---------|
| `x-body-hash` | Request body | `--enable-body-hash` | Disabled |
| `x-path-hash` | Request path (before `?`) | `--enable-path-hash` | Disabled |
| `x-url-hash` | Full URL (path + query string from `:path` header) | `--enable-url-hash` | Disabled |

### ext-proc Envoy Configuration

The ext-proc filter requires specific `processing_mode` settings:

| Mode | Value | Reason |
|------|-------|--------|
| `request_header_mode` | `SEND` | Required to initiate gRPC stream |
| `response_header_mode` | `SKIP` | No response header processing needed |
| `request_body_mode` | `BUFFERED` | Buffer entire body for SHA256 computation |
| `response_body_mode` | `NONE` | No response body processing needed |

**Important**: `request_header_mode: SEND` is mandatory even when only body processing is needed. Without it, Envoy will not initiate the gRPC stream to the ext-proc server.

**Note**: The ext-proc server handles all request types (ResponseHeaders, ResponseBody, RequestTrailers, ResponseTrailers) with empty default responses for robustness. This prevents 504 Gateway Timeout errors when EnvoyFilter is misconfigured with modes like `response_header_mode: SEND`.

When implementing ext-proc in Rust with tonic, use `HeaderValue.raw_value` field (not `value`) for setting header values. The `value` field is silently ignored by Envoy ext-proc.

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

#### Dual Configuration Support
All filters support both WasmPlugin and EnvoyFilter configuration methods using a fallback pattern:
```rust
self.get_plugin_configuration().or_else(|| self.get_vm_configuration())
```

| Configuration Method | Source | Priority |
|---------------------|--------|----------|
| WasmPlugin | `spec.pluginConfig` | Primary (checked first) |
| EnvoyFilter | `vm_config.configuration` | Fallback |

#### When to Use WasmPlugin vs EnvoyFilter

| Use Case | Choice | Reason |
|----------|--------|--------|
| Default | WasmPlugin | Simpler, fewer lines of YAML |
| Need `INSERT_BEFORE` specific filter | EnvoyFilter | WasmPlugin only supports phase-based ordering |
| Need to insert before `istio.metadata_exchange` | EnvoyFilter | WasmPlugin cannot insert before this filter |
| Need precise filter chain control | EnvoyFilter | Full control over filter ordering |

**WasmPlugin example:**
```yaml
apiVersion: extensions.istio.io/v1alpha1
kind: WasmPlugin
spec:
  pluginConfig:
    log_level: debug
    # filter-specific fields
```

**EnvoyFilter example:**
```yaml
apiVersion: networking.istio.io/v1alpha3
kind: EnvoyFilter
spec:
  configPatches:
    - applyTo: EXTENSION_CONFIG
      patch:
        operation: ADD
        value:
          typed_config:
            "@type": type.googleapis.com/envoy.extensions.filters.http.wasm.v3.Wasm
            config:
              vm_config:
                configuration:
                  "@type": type.googleapis.com/google.protobuf.StringValue
                  value: '{"log_level":"debug"}'
```

### Testing Approach
- Unit tests: Standard Rust tests in each package
- Integration tests: K6 scripts in `k6/` directory test filter behavior through HTTP requests
- E2E tests create an isolated Kind cluster with Envoy and the filters deployed

### Deployment
Filters are built into WASM artifacts and served via HTTP (nghttpd on port 8080) or loaded from filesystem. Envoy loads them as HTTP filters in its filter chain configuration.
