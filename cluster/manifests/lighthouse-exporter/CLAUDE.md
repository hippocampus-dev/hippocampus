# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Lighthouse Exporter is a Kubernetes-deployed Node.js application that continuously monitors web performance using Google Lighthouse. It runs audits on specified URLs, collects performance metrics for both desktop and mobile form factors, and exports results as Prometheus metrics.

This directory contains the Kubernetes manifests for deploying the Lighthouse Exporter, while the application code resides in `/opt/hippocampus/cluster/applications/lighthouse-exporter/`.

## Common Development Commands

### Application Development (in `/opt/hippocampus/cluster/applications/lighthouse-exporter/`)
```bash
# Install dependencies
npm install

# Run locally
node main.mjs <urls...> [options]

# Example with options
node main.mjs https://example.com --port 9090 --interval 30000 --form-factors desktop,mobile

# Build Docker image
docker build -t lighthouse-exporter .

# Run via Docker
docker run -p 8080:8080 lighthouse-exporter https://example.com
```

### Kubernetes Deployment (in this directory)
```bash
# Apply base configuration
kubectl apply -k base/

# Apply development overlay
kubectl apply -k overlays/dev/

# Update image digest in base kustomization
cd base && kustomize edit set image ghcr.io/hippocampus-dev/hippocampus/lighthouse-exporter@sha256:<new-digest>

# Build manifests without applying
kustomize build overlays/dev/
```

## Architecture

### Application Structure
The Lighthouse Exporter (`/opt/hippocampus/cluster/applications/lighthouse-exporter/main.mjs`) implements:
- **Sequential Execution**: Mutex-based runner prevents resource conflicts when running multiple Lighthouse audits
- **Metrics Collection**: OpenTelemetry SDK collects and exports metrics to Prometheus format
- **Browser Management**: Puppeteer controls headless Chrome instances with automatic cleanup
- **Distributed Tracing**: Optional traceparent header injection for request correlation

### Kubernetes Deployment Structure
```
manifests/lighthouse-exporter/
├── base/                    # Base Kubernetes resources
│   ├── deployment.yaml      # Core deployment configuration
│   ├── service.yaml         # Service exposing metrics port
│   └── kustomization.yaml   # Base kustomization with image config
└── overlays/
    └── dev/                 # Development environment overlay
        ├── patches/         # Environment-specific patches
        ├── namespace.yaml   # Namespace definition
        ├── network_policy.yaml
        ├── peer_authentication.yaml
        ├── service_entry.yaml
        ├── sidecar.yaml    # Istio sidecar configuration
        └── telemetry.yaml  # Telemetry configuration
```

### Key Metrics Exposed
- `lighthouse_score{category, url, form_factor}` - Category scores (0-100)
- `lighthouse_audit{id, unit, url, form_factor}` - Individual audit values
- `lighthouse_errors_total{code, url, form_factor}` - Error counter

### Security Considerations
The deployment runs with strict security settings:
- Non-root user (UID: 65532)
- Read-only root filesystem
- All Linux capabilities dropped
- Memory-based temporary directories for Chrome