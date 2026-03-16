# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This directory contains Kubernetes manifests for deploying oauth2-proxy using Kustomize. The oauth2-proxy acts as a reverse proxy that provides authentication via GitHub OAuth for services in the kaidotio.dev domain.

## Architecture

The manifests follow a Kustomize overlay structure:
- **base/**: Core Kubernetes resources with minimal configuration
- **overlays/dev/**: Development environment-specific configurations and patches

### Key Components

1. **OAuth2 Proxy Deployment**: Configured as a GitHub OAuth provider for authentication
2. **Istio Integration**: Full service mesh integration with sidecar injection, traffic management, and observability
3. **External Dependencies**: Requires access to github.com and api.github.com for OAuth authentication

## Common Development Commands

### Deployment Commands
```bash
# Build and review the complete manifest
kubectl kustomize overlays/dev

# Apply to cluster
kubectl apply -k overlays/dev

# Delete from cluster
kubectl delete -k overlays/dev
```

### Validation Commands
```bash
# Validate Kustomize configuration
kubectl kustomize overlays/dev --enable-helm | kubectl apply --dry-run=client -f -

# Check current deployment status
kubectl -n oauth2-proxy get all
kubectl -n oauth2-proxy describe deployment oauth2-proxy
```

## Important Configuration Details

### OAuth2 Proxy Configuration
- Provider: GitHub
- Client ID: `d4035d0a159b047b2a2c` (public)
- Client Secret: Stored in `oauth2-proxy` secret (generated from Vault)
- Redirect URL: `https://oauth2-proxy.kaidotio.dev/oauth2/callback`
- Allowed GitHub user: `kaidotio`
- Cookie domain: `kaidotio.dev`

### Istio Configuration
- Sidecar injection enabled with resource limits
- Gateway exposed at `oauth2-proxy.kaidotio.dev`
- ServiceEntry for GitHub API access
- PeerAuthentication and NetworkPolicy configured

### Resource Management
- HorizontalPodAutoscaler configured for scaling
- PodDisruptionBudget for availability during updates
- TopologySpreadConstraints for pod distribution across zones and nodes

## Manifest Checklist Compliance

This deployment follows the cluster-wide manifest checklist (../README.md):
- ✓ No third-party operators
- ✓ No direct upstream manifests
- ✓ Proper namespace configuration with NetworkPolicy
- ✓ Security context and lifecycle management
- ✓ Istio proxy integration (sidecar, telemetry, peer authentication)
- ✓ Topology spread constraints
- ✓ Pod disruption budget
- ✓ Service traffic distribution