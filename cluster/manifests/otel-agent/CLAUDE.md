# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

The otel-agent is a Kubernetes DaemonSet deployment of OpenTelemetry Collector that runs on every node in the cluster to collect traces from all pods. It acts as a local trace collection agent that forwards data to a central otel-collector service.

## Common Development Commands

### Building Manifests
```bash
# Build production manifests
kustomize build base

# Build development environment manifests with overlays
kustomize build overlays/dev

# Apply to cluster (development)
kustomize build overlays/dev | kubectl apply -f -
```

### Validation
```bash
# Validate YAML syntax
kustomize build overlays/dev | kubectl apply --dry-run=client -f -

# Check kustomization structure
kubectl kustomize overlays/dev
```

## High-Level Architecture

### Deployment Model
- **DaemonSet**: Ensures one pod per node for complete cluster coverage
- **Service**: Exposes trace collection endpoints (gRPC:4317, HTTP:14268)
- **ConfigMap**: OpenTelemetry Collector configuration mounted at runtime

### Trace Collection Pipeline
1. **Receivers**: Accept traces via OTLP (gRPC) and Jaeger (Thrift HTTP) protocols
2. **Processors**: Batch traces (8192 max, 200ms timeout) for efficient forwarding
3. **Exporters**: Forward to central otel-collector service via OTLP

### Directory Structure
```
otel-agent/
├── base/                      # Base Kubernetes resources
│   ├── daemon_set.yaml       # DaemonSet with security hardening
│   ├── service.yaml          # ClusterIP service definition
│   ├── pod_disruption_budget.yaml
│   └── kustomization.yaml    # Base kustomization with image management
└── overlays/
    └── dev/                  # Development environment customizations
        ├── files/
        │   └── config.yaml   # OpenTelemetry Collector configuration
        ├── patches/          # JSON patches for base resources
        ├── namespace.yaml    # Namespace with Pod Security Standards
        ├── network_policy.yaml
        └── kustomization.yaml
```

### Key Design Patterns

1. **Security-First**: Runs as non-root (UID 65532), read-only filesystem, all capabilities dropped
2. **Network Isolation**: Default-deny NetworkPolicy with explicit port allowlisting
3. **Service Mesh Integration**: Istio sidecar injection and mTLS enabled
4. **High Availability**: PodDisruptionBudget ensures service continuity during updates
5. **Observability**: Prometheus metrics exposed on port 8888

### Integration Points
- **Upstream**: Applications send traces to otel-agent on each node
- **Downstream**: otel-agent forwards to `otel-collector.otel:4317`
- **Monitoring**: Prometheus scrapes metrics from port 8888
- **Service Mesh**: Istio provides mTLS and distributed tracing context

## Important Configuration Details

### OpenTelemetry Collector Config (`overlays/dev/files/config.yaml`)
- Defines receivers, processors, exporters, and service pipelines
- Mounted as ConfigMap to `/etc/otelcol-contrib/config.yaml`
- Changes require pod restart to take effect

### Image Management
- Uses mirrored images from `ghcr.io/hippocampus-dev/hippocampus/mirror`
- Image digest pinning for reproducible deployments
- Update via kustomization.yaml in base directory

### Resource Limits
- Memory: 512Mi limit, 256Mi request
- CPU: 200m limit, 100m request
- Adjust based on node trace volume

## Development Workflow

1. Modify configuration in `overlays/dev/files/config.yaml` for collector behavior changes
2. Update base resources in `base/` for structural changes
3. Test locally with `kustomize build overlays/dev`
4. Apply to development cluster and verify trace flow
5. Monitor collector metrics at `http://otel-agent:8888/metrics`