# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This directory contains Kubernetes manifests for the reporting-server service, which implements the W3C Reporting API. The service handles various types of security and error reports including CSP violations, network errors, and other policy violations.

The reporting-server is a Go-based microservice that:
- Accepts browser security reports (CSP violations, network errors, etc.)
- Integrates with OpenTelemetry for distributed tracing
- Exposes Prometheus metrics
- Supports continuous profiling with Pyroscope

## Common Development Commands

### Application Development
Navigate to the application source code: `/cluster/applications/reporting-server/`

- `make dev` - Run the service with hot-reload using watchexec
- `go test ./...` - Run all tests
- `go test -run <TestName>` - Run a specific test
- `go build` - Build the binary

### Manifest Management
From this directory (`/cluster/manifests/reporting-server/`):

- `kubectl apply -k overlays/dev/` - Deploy to development environment
- `kubectl apply -k base/` - Deploy base configuration (not recommended, use overlays)

## Architecture and Structure

### Manifest Organization
- `base/` - Core Kubernetes resources:
  - `deployment.yaml` - Main deployment configuration
  - `service.yaml` - Service exposure
  - `horizontal_pod_autoscaler.yaml` - Auto-scaling configuration
  - `pod_disruption_budget.yaml` - High availability settings
  - `kustomization.yaml` - Kustomize base configuration

- `overlays/dev/` - Development environment specific configurations:
  - Environment-specific patches and resource modifications
  - Uses Kustomize to merge with base configurations

### Service Architecture
The reporting-server handles multiple report types:
- **CSP Reports** - Content Security Policy violations
- **Network Error Reports** - Network-level errors as per W3C Network Error Logging
- **Deprecation Reports** - Browser feature deprecation usage
- **Intervention Reports** - Browser intervention reports
- **Crash Reports** - Browser crash information

Key endpoints:
- `/report` - Accepts all W3C Reporting API compliant reports
- `/healthz` - Kubernetes health check
- `/metrics` - Prometheus metrics endpoint
- `/debug/pprof/*` - Go profiling endpoints

### Integration Points
- **OpenTelemetry**: Distributed tracing with OTLP gRPC exporter
- **Prometheus**: Metrics collection and monitoring
- **Pyroscope**: Continuous profiling for performance analysis
- **Istio**: Service mesh integration (virtual services, traffic policies)

## Development Patterns

1. **Environment Configuration**: Uses environment variables for configuration (loaded via godotenv)
2. **Observability First**: Comprehensive telemetry with OpenTelemetry, Prometheus, and Pyroscope
3. **Graceful Shutdown**: Proper signal handling for clean container termination
4. **Resource Limits**: Defined CPU/memory limits for predictable performance
5. **High Availability**: Pod disruption budgets and horizontal pod autoscaling

## Testing and Validation

Before deploying changes:
1. Ensure the Go application builds successfully: `go build`
2. Run tests in the application directory: `go test ./...`
3. Validate Kubernetes manifests: `kubectl apply -k base/ --dry-run=client`
4. Check Kustomize output: `kubectl kustomize overlays/dev/`