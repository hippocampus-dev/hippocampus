# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is an ArgoCD deployment configuration for a Kubernetes cluster that follows GitOps principles. The deployment uses Kustomize for configuration management with a base/overlay pattern, implementing advanced security, observability, and high-availability features.

## Common Development Commands

### Deployment Commands
```bash
# Build and view the dev overlay configuration
kustomize build overlays/dev/

# Apply the dev overlay to a cluster
kubectl apply -k overlays/dev/

# Validate kustomization without applying
kubectl apply -k overlays/dev/ --dry-run=client

# Check ArgoCD pods status after deployment
kubectl -n argocd get pods
kubectl -n argocd logs -l app.kubernetes.io/name=argocd-repo-server
```

### Development and Testing
```bash
# Test the Vault plugin locally (requires Go)
cd overlays/dev/files/secretsfromvault
go mod tidy
go build -buildmode plugin

# Validate YAML syntax
kubectl apply -k overlays/dev/ --dry-run=client --validate=true
```

## High-Level Architecture

### Kustomization Structure
- **Base Layer**: References official ArgoCD v2.8.4 manifests from GitHub
- **Dev Overlay**: Adds environment-specific customizations including:
  - Istio service mesh integration (sidecars, gateways, virtual services)
  - Vault secrets management via custom Kustomize plugin
  - Redis high-availability configuration
  - Comprehensive network policies (zero-trust)
  - Security hardening (non-root, read-only filesystems, dropped capabilities)

### Key Architectural Patterns

1. **Custom Kustomize Build**: The repo-server uses an init container to build Kustomize v5.5.0 with a custom Vault plugin (`secretsfromvault`). This plugin:
   - Authenticates to Vault using Kubernetes service account tokens
   - Retrieves secrets from Vault at build time
   - Supports data URLs for binary secrets
   - Built as a Go plugin and mounted into the repo-server

2. **Image Mirroring**: All container images are pulled from a private registry (`ghcr.io/hippocampus-dev/hippocampus/mirror/`) for security and reliability

3. **Network Security**:
   - Default-deny NetworkPolicies with explicit allow rules per component
   - Istio mTLS via PeerAuthentication
   - Service mesh integration for all ArgoCD components

4. **High Availability**:
   - Pod disruption budgets for all components
   - Topology spread constraints for zone/node distribution
   - Redis clustering with HAProxy for connection management
   - Configurable replicas with automatic environment variable updates

5. **GitOps Configuration**:
   - Repository: `git@github.com:hippocampus-dev/hippocampus`
   - SSH key authentication (stored in `argocd-credentials` secret)
   - Custom Kustomize with alpha plugins enabled
   - Self-healing enabled with 60-second timeout

### Security Features
- Non-root containers (UIDs: 65532, 999, 1337)
- Read-only root filesystems
- Seccomp profiles enabled
- All capabilities dropped
- Istio sidecar injection with resource limits
- Vault integration for secret management

### Development Environment Specifics
- Insecure mode enabled (`server.insecure: "true"`)
- Authentication disabled (`server.disable.auth: "true"`)
- Local development domains (`*.minikube.127.0.0.1.nip.io`)
- Prometheus metrics exposed on all components