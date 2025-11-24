# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Kubernetes metrics-server deployment using Kustomize. The metrics-server is a critical component that collects resource metrics from Kubelets and exposes them via the Metrics API for use by Horizontal Pod Autoscaler and Vertical Pod Autoscaler.

## Architecture

### Deployment Structure
- **Dual Deployment Pattern**: Two separate deployments (`metrics-server-a` and `metrics-server-b`) for high availability
- **Kustomize-based**: Uses base manifests with environment-specific overlays (e.g., dev)
- **Components**:
  - metrics-server container: Core metrics collection
  - addon-resizer container: Dynamically adjusts resource allocation based on cluster size

### Key Design Decisions
1. **High Availability**: Two deployments (a/b) ensure metrics collection continues if one fails
2. **Security Hardening**:
   - Non-root user (65532)
   - Read-only root filesystem
   - All capabilities dropped
   - Seccomp profiles enforced
3. **Resource Management**: addon-resizer automatically scales resources based on cluster nodes
4. **Topology Spread**: Pods distributed across nodes and zones for resilience

## Development Commands

### Applying Manifests
```bash
# Apply dev overlay
kubectl apply -k overlays/dev/

# Apply base manifests directly
kubectl apply -k base/
```

### Validation
```bash
# Validate kustomization
kubectl kustomize overlays/dev/ | kubectl apply --dry-run=client -f -

# Check deployment status
kubectl -n kube-system get deploy metrics-server-a metrics-server-b
kubectl -n kube-system get pods -l app.kubernetes.io/name=metrics-server
```

### Testing Metrics API
```bash
# Test metrics availability
kubectl top nodes
kubectl top pods --all-namespaces
```

## Important Configuration

### Key Arguments
- `--kubelet-insecure-tls`: Allows connection to kubelets without TLS verification (common in dev/test)
- `--metric-resolution=30s`: Metrics collection interval
- `--kubelet-use-node-status-port`: Uses kubelet's read-only port for metrics

### Image Management
Images are mirrored to `ghcr.io/kaidotio/hippocampus/mirror/` for consistency and availability control.

### Resource Scaling (addon-resizer)
Default scaling parameters in dev overlay:
- Base: 100m CPU, 200Mi memory
- Per-node increment: 1m CPU, 2Mi memory