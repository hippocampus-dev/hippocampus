# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`prometheus-metrics-proxy-hook` is a Kubernetes admission webhook that injects a metrics proxy sidecar container when pods have `prometheus.io/wait` annotation. The sidecar proxies metrics requests and tracks the last scrape time, ensuring metrics are scraped before pod termination.

## Common Development Commands

### Local Development
- `make dev` - Runs Skaffold in development mode with port forwarding for hot-reload development
- `skaffold dev --port-forward` - Same as above, direct Skaffold command

### Building
- `go build` - Build the binary locally
- `docker build -t prometheus-metrics-proxy-hook .` - Build Docker image

### Testing
- `go test ./...` - Run tests (note: no tests currently exist)

### Go Module Management
- `go mod tidy` - Clean up dependencies
- `go mod download` - Download dependencies

## High-Level Architecture

### Components

1. **Webhook Server** (`cmd/webhook.go`, `pkg/webhook/`)
   - Kubernetes admission webhook that intercepts pod creation
   - Checks for `prometheus.io/wait` annotation
   - Rewrites `prometheus.io/port` to point to the proxy
   - Injects metrics proxy sidecar container

2. **Sidecar** (`cmd/sidecar.go`, `pkg/sidecar/`)
   - Runs alongside application containers as a reverse proxy
   - Proxies requests to the original metrics endpoint
   - Records the timestamp of each scrape
   - On SIGTERM, waits for final scrape if recently scraped (within 1 second)
   - Ensures metrics increments are not lost during pod termination

### How It Works

1. Pods annotated with `prometheus.io/wait: "true"` trigger the webhook
2. The webhook reads the original metrics port from `prometheus.io/port` (default: 9090)
3. A new proxy port is calculated (original + 10000, or 19090 fallback)
4. The `prometheus.io/port` annotation is rewritten to point to the proxy
5. A sidecar container is injected that:
   - Listens on the proxy port
   - Forwards requests to localhost:originalPort
   - Records scrape timestamps
   - Waits for final scrape on termination

### Key Annotations

Pods must have these annotations:
- `prometheus.io/wait: "true"` - Enable the metrics proxy
- `prometheus.io/port: "9090"` - Original metrics port (optional, default: 9090)

The webhook adds:
- `prometheus.io/original-port` - Stores the original port for reference

### Command Line Arguments

#### Webhook-specific Flags
- `--host` - Webhook server host (default: 0.0.0.0)
- `--port` - Webhook server port (default: 9443)
- `--certDir` - TLS certificate directory
- `--sidecar-image` - Docker image for sidecar containers
- `--enable-sidecar-containers` - Use Kubernetes sidecar containers feature
- `--metrics-bind-address` - Metrics endpoint address
- `--health-probe-bind-address` - Health probe endpoint address

#### Sidecar-specific Flags
- `--original-port` - Original metrics port to proxy (default: 9090)
- `--proxy-port` - Port for the proxy to listen on (default: 19090)
- `--termination-grace-period-seconds` - Grace period for shutdown (default: 30)

### Deployment

The webhook is typically deployed using:
1. Kubernetes Deployment with TLS certificates (cert-manager)
2. MutatingWebhookConfiguration to intercept pod creation
3. Service to expose the webhook
4. RBAC permissions for reading pods

### Benefits

- **Zero metric loss** - Ensures all incremented metrics are scraped before pod termination
- **Transparent** - Works with any Prometheus-scraped application
- **Configurable** - Grace period and ports can be customized
- **Minimal overhead** - Simple reverse proxy with timestamp tracking