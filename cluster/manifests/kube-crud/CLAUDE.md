# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This directory contains Kubernetes manifests for the kube-crud application, a web-based frontend for managing Kubernetes resources (currently focused on CronJobs). The manifests use Kustomize for environment-specific configuration management and integrate with Istio service mesh.

## Common Development Commands

### Deployment Commands
- `kubectl apply -k overlays/dev/` - Deploy to development environment
- `kubectl apply -k base/` - Deploy base configuration (not recommended for direct use)
- `kubectl -n kube-crud get pods` - Check pod status
- `kubectl -n kube-crud logs -f deployment/kube-crud` - View application logs
- `kubectl -n kube-crud port-forward deployment/kube-crud 8080:8080` - Local port forwarding for testing

### Kustomize Operations
- `kubectl kustomize overlays/dev/` - Preview generated manifests for dev environment
- `kubectl kustomize base/` - Preview base manifests
- `kustomize edit set image ghcr.io/hippocampus-dev/hippocampus/kube-crud=<new-tag>` - Update image tag in base

### Development Workflow
- `skaffold dev --port-forward` - Continuous deployment with hot reload (from parent directory)
- Modify manifests in `base/` for application-wide changes
- Modify manifests in `overlays/dev/` for environment-specific changes

## High-Level Architecture

### Manifest Structure
```
kube-crud/
├── base/                          # Base Kustomize configuration
│   ├── deployment.yaml           # Core deployment manifest
│   ├── horizontal_pod_autoscaler.yaml  # HPA configuration
│   ├── kustomization.yaml        # Base kustomization
│   ├── pod_disruption_budget.yaml # PDB for high availability
│   └── service.yaml              # ClusterIP service
└── overlays/
    └── dev/                      # Development environment overlay
        ├── files/                # ConfigMap files
        │   └── host.js          # API endpoint configuration
        ├── gateway.yaml         # Istio Gateway configuration
        ├── kustomization.yaml   # Dev kustomization
        ├── namespace.yaml       # Namespace definition
        ├── network_policy.yaml  # Network access controls
        ├── patches/             # JSON patches for base resources
        ├── peer_authentication.yaml # Istio mTLS configuration
        ├── sidecar.yaml         # Istio sidecar configuration
        ├── telemetry.yaml       # Istio telemetry configuration
        └── virtual_service.yaml # Istio routing rules
```

### Key Configuration Patterns

1. **Security Configuration**
   - Non-root user (65532) with read-only root filesystem
   - No service account token automounting
   - All capabilities dropped
   - RuntimeDefault seccomp profile
   - Memory-based temporary volumes for Nginx

2. **Istio Integration**
   - Gateway exposes application on `kube-crud.hippocampus.server`
   - Strict mTLS within namespace
   - Custom telemetry for access logs
   - Sidecar configuration for optimized proxy behavior

3. **Kustomize Patterns**
   - Base contains minimal viable configuration
   - Overlays add environment-specific resources
   - ConfigMapGenerator creates immutable ConfigMaps from files
   - Strategic merge patches modify base resources

4. **High Availability**
   - HPA scales 1-3 replicas based on CPU/memory
   - PodDisruptionBudget ensures at least 1 pod during disruptions
   - Readiness probe with custom success threshold

### Important Implementation Details

- **Image Management**: Base kustomization tracks current image digest for reproducible deployments
- **ConfigMap Generation**: `host.js` is injected via ConfigMap volume mount (replaces build-time configuration)
- **Network Policies**: Restricts ingress to Istio system namespace only
- **Resource Limits**: Set via patches in overlays to allow environment-specific sizing
- **Health Checks**: `/healthz` endpoint with aggressive readiness probe (1s interval)