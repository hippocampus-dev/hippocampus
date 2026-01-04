# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Component Overview

anonymous-proxy is a Go-based proxy that provides anonymous access to the Kubernetes API server's OIDC discovery endpoint (`/openid/v1/jwks`). It uses the pod's service account token to authenticate with the Kubernetes API server, allowing external clients to access the JWKS endpoint without authentication.

## Common Development Commands

### Deployment Commands
```bash
# View generated manifests for dev environment
kubectl kustomize overlays/dev

# Deploy to cluster
kubectl apply -k overlays/dev

# Remove from cluster
kubectl delete -k overlays/dev

# Check deployment status
kubectl -n anonymous-proxy get all

# View logs
kubectl -n anonymous-proxy logs -l app.kubernetes.io/name=anonymous-proxy
```

### Image Management
```bash
# Update image digest in applications manifests
cd /opt/hippocampus/cluster/applications/anonymous-proxy/manifests
kustomize edit set image ghcr.io/hippocampus-dev/hippocampus/anonymous-proxy=ghcr.io/hippocampus-dev/hippocampus/anonymous-proxy@sha256:new-digest
```

### Local Testing
```bash
# Port-forward for testing
kubectl -n anonymous-proxy port-forward service/anonymous-proxy 8080:8080

# Test JWKS endpoint
curl http://localhost:8080/openid/v1/jwks

# Health check
curl http://localhost:8080/healthz
```

## High-Level Architecture

### Kustomize Structure
- **base/**: References core manifests from `/opt/hippocampus/cluster/applications/anonymous-proxy/manifests/`
- **overlays/dev/**: Development environment with Istio integration, network policies, and resource tuning

### Key Design Patterns
1. **Security-hardened deployment**: Non-root user (65532), read-only filesystem, dropped capabilities
2. **High availability**: HPA (1-5 replicas), PDB, topology spread constraints
3. **Istio integration**: mTLS, telemetry, sidecar injection in dev overlay
4. **Zero-trust networking**: Default-deny network policies with explicit ingress/egress rules
5. **Graceful shutdown**: Lameduck period and termination grace period handling