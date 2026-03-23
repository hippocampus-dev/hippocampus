# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This directory contains Kustomize manifests for deploying the Kubernetes Gateway API to the Hippocampus platform. Gateway API is a collection of resources that model service networking in Kubernetes, providing a more expressive and extensible alternative to Ingress.

The manifests deploy Gateway API v1.1.0 CRDs and controllers to enable advanced traffic management capabilities across the cluster.

## Common Development Commands

### Deployment Commands
```bash
# Deploy Gateway API to development environment
kubectl apply -k overlays/dev/

# View deployed Gateway API resources
kubectl get crd | grep gateway.networking.k8s.io
kubectl get gatewayclass
kubectl get gateway -A

# Check ArgoCD application status
kubectl get application gateway-api -n argocd
kubectl describe application gateway-api -n argocd

# Manually sync ArgoCD application
kubectl patch application gateway-api -n argocd --type merge -p '{"operation": {"sync": {}}}'
```

### Validation Commands
```bash
# Validate kustomize build without applying
kubectl kustomize overlays/dev/ | kubectl apply --dry-run=client -f -

# Build and inspect the final manifests
kubectl kustomize overlays/dev/
```

## High-Level Architecture

### Directory Structure
```
gateway-api/
├── base/                    # Base Gateway API configuration
│   └── kustomization.yaml  # References upstream Gateway API standard install
└── overlays/
    └── dev/                # Development environment overlay
        └── kustomization.yaml  # Applies namespace and includes base
```

### Key Design Patterns

1. **Upstream Integration**: Uses official Gateway API manifests directly from kubernetes-sigs/gateway-api releases
2. **Namespace Isolation**: Deploys to `kube-system` namespace for cluster-wide availability
3. **ArgoCD Management**: Deployed via ArgoCD with sync-wave -50 (early deployment priority)
4. **Kustomize Overlays**: Follows standard kustomize pattern for environment-specific configuration

### Integration Points

- **Sync Wave**: Set to -50 in ArgoCD, ensuring Gateway API CRDs are installed before services that depend on them
- **Standard Channel**: Uses the standard install which includes stable Gateway API resources
- **Version**: Currently pinned to v1.1.0 of Gateway API

## Important Notes

- Gateway API CRDs must be installed before any services using Gateway/HTTPRoute resources
- Changes to the Gateway API version require updating the URL in base/kustomization.yaml
- This manifest installs only the CRDs; actual Gateway controller implementations (like Istio, Envoy Gateway) are deployed separately