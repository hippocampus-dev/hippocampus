# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This directory contains Kustomize manifests for deploying the Kubernetes Volume Snapshot Controller in the Hippocampus cluster. The snapshot-controller is a standard Kubernetes component that manages VolumeSnapshot resources and integrates with CSI drivers to create, delete, and restore volume snapshots.

## Architecture

### Directory Structure
```
├── base/
│   └── kustomization.yaml          # References upstream snapshot-controller manifests
└── overlays/
    └── dev/
        ├── kustomization.yaml      # Dev overlay configuration
        ├── namespace.yaml          # Creates snapshot-controller namespace
        ├── patches/
        │   ├── deployment.yaml     # Configures replicas, topology spread, Istio sidecar
        │   └── pod_disruption_budget.yaml
        └── [istio configs]         # Service mesh integration files
```

### Kustomize Layering
- **Base**: References `../../../applications/snapshot-controller/manifests` (the upstream application manifests)
- **Overlays/dev**: Adds environment-specific configuration including:
  - Namespace creation with pod-security.kubernetes.io/enforce: restricted
  - Deployment patches for high availability (2 replicas, topology spread constraints)
  - Istio service mesh integration (mTLS, sidecars, telemetry)
  - Network policies (default deny, allow Prometheus scraping on port 15020)

### Istio Service Mesh Integration

The snapshot-controller is fully integrated with Istio:
- **PeerAuthentication** (peer_authentication.yaml:1): Enforces STRICT mTLS for all traffic
- **Sidecar** (sidecar.yaml:1): Configures egress to etcd, Kubernetes API, OTel agent, and istiod
- **Telemetry** (telemetry.yaml:1): 100% tracing, 15s metrics reporting, Envoy access logs
- **NetworkPolicy** (network_policy.yaml:1): Default deny + allow Prometheus on port 15020

### High Availability Configuration

Deployment patch (patches/deployment.yaml:6) configures:
- 2 replicas with RollingUpdate strategy (maxUnavailable: 1)
- Topology spread across nodes and zones with maxSkew: 1
- PodDisruptionBudget allowing maxUnavailable: 1
- Istio sidecar resource limits and Prometheus scraping annotations

### Resource Requirements

The snapshot-controller container requires significant memory for Chromium + Playwright + Xvfb:
- Memory requests: 128Mi
- Memory limits: 512Mi

The 512Mi limit is required to avoid CrashLoopBackOff due to OOM kills during screenshot processing.

## Common Commands

### Validate Manifests
```bash
# Build and preview the complete manifest
kustomize build overlays/dev/

# Validate with kubectl
kustomize build overlays/dev/ | kubectl apply --dry-run=server -f -
```

### Deploy Changes
```bash
# Apply via kubectl (if not using ArgoCD)
kustomize build overlays/dev/ | kubectl apply -f -

# Or let ArgoCD sync (recommended - note sync-wave: -1 annotation)
kubectl apply -f <argocd-application-manifest>
```

### Debugging
```bash
# Check controller status
kubectl get deployment -n snapshot-controller
kubectl get pods -n snapshot-controller
kubectl logs -n snapshot-controller -l app.kubernetes.io/name=snapshot-controller

# Verify VolumeSnapshot CRDs
kubectl get crd | grep snapshot

# Check Istio sidecar status
kubectl get pods -n snapshot-controller -o jsonpath='{.items[*].spec.containers[*].name}'
```

## Key Configuration Details

### ArgoCD Sync Wave
The `generatorOptions.annotations` in overlays/dev/kustomization.yaml:19 sets `argocd.argoproj.io/sync-wave: "-1"`, ensuring this deploys before most other applications (VolumeSnapshot CRDs must exist before workloads can use them).

### Network Access
The snapshot-controller needs access to (see sidecar.yaml:14):
- Kubernetes API server (default/kubernetes.default.svc.cluster.local)
- etcd (default/etcd.default.svc.cluster.local)
- OpenTelemetry agent (otel/otel-agent.otel.svc.cluster.local)
- Istio control plane (istio-system/istiod.istio-system.svc.cluster.local)

### Security Posture
- Namespace enforces "restricted" pod security standard
- Default deny network policy with explicit allowlist
- STRICT mTLS for all service-to-service communication
- No ingress except Prometheus metrics scraping

## Modification Guidelines

When modifying these manifests:
1. Changes to base upstream manifests should be done in `cluster/applications/snapshot-controller/manifests`
2. Environment-specific patches go in `overlays/dev/patches/`
3. New Istio configurations should follow the existing pattern (selector matching `app.kubernetes.io/name: snapshot-controller`)
4. Maintain topology spread constraints for zone-level high availability
5. Preserve the sync-wave annotation to ensure proper deployment ordering
