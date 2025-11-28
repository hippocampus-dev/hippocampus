# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

This directory contains Kubernetes manifests for httpbin, a HTTP request & response testing service. It's deployed as part of the Hippocampus platform with two variants:
- **httpbin**: Standard deployment without Istio sidecar
- **httpbin-istio**: Deployment with Istio service mesh integration

## Common Development Commands

### Kubernetes Operations
```bash
# Build and preview manifests
kustomize build overlays/dev

# Apply to cluster
kubectl apply -k overlays/dev

# Check resources
kubectl get all -n httpbin
kubectl describe deployment httpbin -n httpbin
kubectl logs -f deployment/httpbin -n httpbin

# Port forward for local testing
kubectl port-forward -n httpbin service/httpbin 8000:8000
```

### Testing the Service
```bash
# After port-forwarding
curl http://localhost:8000/status/200
curl http://localhost:8000/headers
curl -X POST http://localhost:8000/post -d "data=test"
```

## Architecture

### Directory Structure
```
httpbin/
├── base/
│   └── kustomization.yaml         # References ../../utilities/httpbin
└── overlays/
    └── dev/
        ├── kustomization.yaml     # Main overlay configuration
        ├── namespace.yaml         # Namespace definition
        ├── network_policy.yaml    # Network access control
        ├── patches/               # Resource modifications
        │   ├── deployment.yaml
        │   ├── horizontal_pod_autoscaler.yaml
        │   ├── pod_disruption_budget.yaml
        │   └── service.yaml
        └── [Istio configuration files]
```

### Key Components

1. **Base Resources** (from utilities/httpbin):
   - Two deployments: httpbin and httpbin-istio
   - Services for each deployment
   - HorizontalPodAutoscaler definitions
   - PodDisruptionBudget for availability

2. **Security Configuration**:
   - Non-root user (UID: 65532)
   - Read-only root filesystem
   - All capabilities dropped
   - Seccomp profile enforced
   - NetworkPolicy for traffic control

3. **Istio Integration** (in httpbin-istio):
   - Gateway for external access
   - VirtualService for routing
   - AuthorizationPolicy for access control
   - PeerAuthentication for mTLS
   - WasmPlugin for extensions
   - Telemetry configuration

### Configuration Patterns

When modifying httpbin manifests:

1. **Patches**: Use patches in overlays/dev/patches/ for environment-specific changes
2. **Resource Limits**: Define in deployment patch, not base
3. **Replicas**: Set in HPA patch (min/max), not deployment
4. **Topology**: Configure spread constraints in deployment patch
5. **Istio Settings**: Modify sidecar resources via annotations

### Common Modifications

**Adding Resource Limits**:
Edit `overlays/dev/patches/deployment.yaml` to add resource constraints

**Adjusting Autoscaling**:
Edit `overlays/dev/patches/horizontal_pod_autoscaler.yaml` to set min/max replicas and metrics

**Changing Service Type**:
Edit `overlays/dev/patches/service.yaml` to modify service configuration

**Istio Traffic Management**:
Edit `overlays/dev/virtual_service.yaml` for routing rules

## Important Notes

- Container runs as non-root user with strict security context
- Uses memory-backed /tmp volume for read-only filesystem compatibility
- Graceful shutdown configured with 3-second preStop hook
- Image is mirrored to GitHub Container Registry for reliability
- Both deployments share the same container configuration but differ in Istio sidecar injection