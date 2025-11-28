# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

### Deploy cert-manager
```bash
# Deploy to dev environment
kubectl apply -k overlays/dev/

# Check deployment status
kubectl -n cert-manager get pods
kubectl -n cert-manager get deployments
```

### Manage certificates
```bash
# List all cert-manager resources
kubectl get certificates,certificaterequests,issuers,clusterissuers --all-namespaces

# Check certificate status
kubectl describe certificate <name> -n <namespace>

# View cert-manager logs
kubectl -n cert-manager logs -l app.kubernetes.io/name=cert-manager
```

### Update images
When updating cert-manager versions, update the image digests in `base/kustomization.yaml`. All images must be mirrored to `ghcr.io/kaidotio/hippocampus/mirror/`.

## Architecture

This repository contains Kubernetes manifests for cert-manager using Kustomize:

**Base manifests** (`/base/`):
- Core cert-manager components: controller, webhook, cainjector
- RBAC resources and CRDs
- Uses mirrored images from the Hippocampus registry

**Dev overlay** (`/overlays/dev/`):
- Scales all components to 2 replicas for HA
- Enables Istio sidecar injection with resource limits
- Adds topology spread constraints for zone/node distribution
- Configures Prometheus metrics scraping on the controller
- Implements network policies and mTLS via PeerAuthentication

Key patterns:
- All containers run as non-root with read-only filesystems
- Pod Security Standards enforced on namespace
- Lifecycle hooks ensure graceful shutdown
- Service mesh integration for observability and security