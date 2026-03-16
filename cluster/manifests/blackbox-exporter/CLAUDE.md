# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Blackbox Exporter Kubernetes manifests for probing endpoints over HTTP, HTTPS, DNS, TCP, and ICMP. This is deployed as a Prometheus exporter that performs black-box monitoring of external and internal services.

## Common Development Commands

### Deployment
```bash
# Deploy via ArgoCD (automatic GitOps)
# Defined in: cluster/manifests/argocd-applications/base/blackbox-exporter.yaml

# Manual deployment to dev environment
kubectl apply -k overlays/dev/

# Generate manifests with Kustomize
kubectl kustomize overlays/dev/

# Dry-run deployment
kubectl apply -k overlays/dev/ --dry-run=client
```

### Validation
```bash
# Validate Kustomize configuration
kubectl kustomize overlays/dev/ | kubectl apply --dry-run=client -f -

# Check deployed resources
kubectl -n blackbox-exporter get all

# View blackbox exporter logs
kubectl -n blackbox-exporter logs -l app.kubernetes.io/name=blackbox-exporter
```

### Configuration Testing
```bash
# Test HTTP probe module
kubectl -n blackbox-exporter exec deployment/blackbox-exporter -- wget -qO- 'http://localhost:9115/probe?module=http_2xx&target=http://example.com'

# Check health endpoint
kubectl -n blackbox-exporter exec deployment/blackbox-exporter -- wget -qO- http://localhost:9115/health
```

## High-Level Architecture

### Directory Structure
```
blackbox-exporter/
├── base/                     # Core Kustomize base resources
│   ├── deployment.yaml       # Blackbox exporter Deployment
│   ├── service.yaml          # Service exposing HTTP endpoint
│   ├── config_map.yaml       # Probe module configurations
│   └── kustomization.yaml    # Base Kustomize config with image mirroring
└── overlays/
    └── dev/                  # Development environment customizations
        ├── patches/          # Deployment and service modifications
        │   ├── deployment.yaml  # Adds replicas, Istio sidecar, topology spread
        │   └── service.yaml     # Adds trafficDistribution: PreferClose
        ├── namespace.yaml
        ├── network_policy.yaml
        ├── peer_authentication.yaml
        ├── sidecar.yaml
        └── telemetry.yaml
```

### Key Configuration Patterns

1. **Security-hardened**:
   - Runs as non-root user (UID 65532)
   - Read-only root filesystem
   - All capabilities dropped
   - RuntimeDefault seccomp profile

2. **Probe Modules** (defined in config_map.yaml):
   - `http_2xx` - HTTP/HTTPS GET requests expecting 2xx status
   - `http_post_2xx` - HTTP POST requests with JSON content type
   - `tcp_connect` - TCP connectivity checks
   - `dns_udp` - DNS resolution over UDP
   - `icmp` - ICMP ping probes (requires privileged mode in some environments)

3. **Istio Integration**:
   - Service mesh sidecar injection enabled
   - Prometheus scraping on port 9115 via annotations
   - mTLS peer authentication
   - Resource limits for Envoy sidecar: 30m/64Mi requests, 1000m/1Gi limits

4. **High Availability**:
   - Topology spread constraints for even distribution across nodes and zones
   - Rolling update strategy (25% surge, 1 max unavailable)
   - readinessProbe ensures traffic only to healthy pods

5. **Image Mirroring**: Uses `ghcr.io/hippocampus-dev/hippocampus/mirror/quay.io/prometheus/blackbox-exporter` with digest pinning

### Deployment Workflow

1. Base manifests define core Deployment, Service, and ConfigMap with security settings
2. Dev overlay adds namespace (`blackbox-exporter`), Istio integration, and environment patches
3. Kustomize handles image transformations and resource merging
4. Typically deployed via ArgoCD for GitOps workflow

## Important Notes

- The exporter itself does not perform probes automatically - it must be configured as a scrape target in Prometheus with appropriate `module` and `target` parameters
- ICMP probes may require additional capabilities or NET_RAW permission depending on kernel version
- All probe timeout is set to 5 seconds
- Network policies allow Prometheus to scrape metrics from the `prometheus` namespace
