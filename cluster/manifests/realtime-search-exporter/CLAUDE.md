# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

The realtime-search-exporter Kubernetes manifests provide deployment configuration for the realtime-search-exporter microservice. This service monitors Yahoo Japan's real-time search results for specified keywords and exports Prometheus metrics about their frequency in recent tweets.

## Common Development Commands

### Application Development (in /opt/hippocampus/cluster/applications/realtime-search-exporter/)
- `make dev` - Starts development server with watchexec for auto-reload
- `make install` - Installs dependencies via UV and Playwright browser
- `uv run -- python main.py <keywords>` - Run the exporter with keywords
- `docker build -t realtime-search-exporter .` - Build Docker image

### Kubernetes Deployment
- `kubectl apply -k base/` - Deploy base configuration
- `kubectl apply -k overlays/dev/` - Deploy with dev overlay
- `kubectl delete -k base/` - Remove deployment

## Architecture

### Manifest Structure
```
manifests/realtime-search-exporter/
├── base/                    # Base Kubernetes resources
│   ├── deployment.yaml      # Main deployment configuration
│   ├── service.yaml         # Service exposure
│   └── kustomization.yaml   # Kustomize base configuration
└── overlays/dev/           # Development environment overlay
    ├── patches/            # Environment-specific modifications
    └── *.yaml             # Additional dev resources (namespace, network policies, etc.)
```

### Key Deployment Features
- **Security Context**: Runs as non-root user (UID 65532) with read-only root filesystem
- **Resource Limits**: CPU and memory limits configured for stability
- **Health Checks**: Readiness probe on metrics endpoint
- **Service Mesh**: Istio sidecar injection enabled in dev overlay
- **Network Policies**: Restricted ingress/egress in dev environment

### Environment Configuration
The deployment accepts these environment variables:
- `HTTP_PROXY` - HTTP proxy for browser requests
- Command arguments specify keywords to monitor

### Metrics Endpoint
- Service exposes port 8080 for Prometheus scraping
- Metrics path: `/metrics`
- Key metric: `keyword_appears_per_hour{keyword="..."}`

## Development Workflow

1. **Modify Application**: Changes to `/opt/hippocampus/cluster/applications/realtime-search-exporter/main.py`
2. **Build Image**: Update Dockerfile and build new container image
3. **Update Manifests**: Modify deployment.yaml with new image tag
4. **Deploy**: Use kubectl with Kustomize to apply changes
5. **Monitor**: Check metrics endpoint and logs

## Notes on Kustomize Usage

- Base configuration is minimal and production-ready
- Dev overlay adds:
  - Dedicated namespace
  - Network policies for security
  - Istio service mesh integration
  - Telemetry configuration
  - Resource adjustments for development