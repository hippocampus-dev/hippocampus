# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is the Prometheus Adapter deployment for the Hippocampus Kubernetes platform. It bridges Prometheus metrics to Kubernetes Custom Metrics API and External Metrics API, enabling Horizontal Pod Autoscaler (HPA) to scale based on application-specific metrics.

## Common Development Commands

### Building and Applying Manifests
```bash
# Build manifests for dev environment
kustomize build overlays/dev

# Apply to Kubernetes cluster
kubectl apply -k overlays/dev

# Dry-run to preview changes
kubectl apply -k overlays/dev --dry-run=client -o yaml

# Validate API service registration
kubectl get apiservice v1beta1.custom.metrics.k8s.io
kubectl get apiservice v1beta1.external.metrics.k8s.io

# Check deployment status
kubectl -n mimir get deployment prometheus-adapter
kubectl -n mimir logs -l app.kubernetes.io/name=prometheus-adapter
```

### Testing Metrics Queries
```bash
# Test custom metrics API
kubectl get --raw "/apis/custom.metrics.k8s.io/v1beta1" | jq

# Query specific metric
kubectl get --raw "/apis/custom.metrics.k8s.io/v1beta1/namespaces/*/pods/*/container_network_receive_bytes_per_second" | jq

# Test external metrics API
kubectl get --raw "/apis/external.metrics.k8s.io/v1beta1" | jq
```

## High-Level Architecture

### Directory Structure
- **base/** - Core Kubernetes resources with default configuration
  - `files/config.yaml` - Metric transformation rules (the heart of the adapter)
  - Standard Kubernetes resources (deployment, service, RBAC)
- **overlays/dev/** - Development environment customizations
  - Istio service mesh integration (mTLS, sidecar, telemetry)
  - Environment-specific patches

### Key Components

1. **Metric Transformation Rules** (`base/files/config.yaml`):
   - Container network metrics: Converts `_total` counters to `_per_second` rates
   - Fluentd buffer queue: Monitors buffer queue length over 2m window
   - GitHub Actions: Exposes queued runs count as external metric
   - Redis events: Publishes Redis script values
   - Cortex query scheduler: Tracks inflight requests (p75 over 2m)

2. **Security Configuration**:
   - TLS certificate generation via init container (cfssl)
   - Non-root execution (UID 65532)
   - Read-only root filesystem
   - Minimal capabilities (drop ALL)
   - Istio mTLS enabled in dev overlay

3. **Deployment Characteristics**:
   - Runs in `mimir` namespace (dev overlay)
   - `system-cluster-critical` priority class
   - Memory-based temporary volumes
   - Graceful shutdown (3s preStop)
   - Health checks on `/healthz`

### Integration Points
- **Prometheus**: Queries metrics from Prometheus server in the cluster
- **Kubernetes API**: Registers as custom/external metrics API provider
- **HPA**: Provides metrics for autoscaling decisions
- **Istio**: Participates in service mesh with mTLS and telemetry

### Adding New Metrics
1. Edit `base/files/config.yaml`
2. Add rule to appropriate section (rules for custom metrics, externalRules for external)
3. Define seriesQuery, resources mapping, name transformation, and metricsQuery
4. Rebuild and apply: `kubectl apply -k overlays/dev`
5. Verify with: `kubectl -n mimir rollout restart deployment prometheus-adapter`

### Troubleshooting
```bash
# Check adapter logs
kubectl -n mimir logs -l app.kubernetes.io/name=prometheus-adapter -f

# Verify metric discovery
kubectl get --raw "/apis/custom.metrics.k8s.io/v1beta1" | jq '.resources[].name' | sort

# Test specific metric query
kubectl get --raw "/apis/custom.metrics.k8s.io/v1beta1/namespaces/<namespace>/pods/<pod>/container_network_receive_bytes_per_second"

# Check HPA using the metrics
kubectl get hpa -A
```