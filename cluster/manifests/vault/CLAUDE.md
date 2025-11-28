# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This directory contains Kubernetes manifests for deploying HashiCorp Vault as part of the Hippocampus platform. Vault runs in dev mode with file-based storage, providing secrets management capabilities primarily for ArgoCD's repository server.

## Common Development Commands

### Deployment Commands
- `kubectl apply -k overlays/dev/` - Deploy Vault to development environment
- `kubectl kustomize overlays/dev/` - Preview generated manifests without applying
- `kubectl delete -k overlays/dev/` - Remove Vault deployment
- `kubectl rollout restart statefulset/vault -n vault` - Restart Vault pods

### Operational Commands
- `kubectl logs -n vault vault-0` - View Vault logs
- `kubectl exec -n vault vault-0 -- vault status` - Check Vault seal status
- `kubectl exec -n vault vault-0 -- cat /data/rootToken` - Get root token (dev only)
- `kubectl exec -n vault vault-0 -- cat /data/unsealKeys` - Get unseal keys (dev only)

### Debugging Commands
- `kubectl port-forward -n vault svc/vault 8200:8200` - Access Vault API locally
- `kubectl describe statefulset -n vault vault` - Check StatefulSet status
- `kubectl get pvc -n vault` - Check persistent volume claims

## High-Level Architecture

### Directory Structure
```
vault/
├── base/                      # Core Kustomize base resources
│   ├── files/                # Configuration files
│   │   ├── config.hcl       # Vault server configuration
│   │   └── init.sh          # Auto-initialization script
│   ├── cluster_role_binding.yaml
│   ├── pod_disruption_budget.yaml
│   ├── service.yaml
│   ├── service_account.yaml
│   ├── stateful_set.yaml
│   └── kustomization.yaml
└── overlays/
    └── dev/                  # Development environment customizations
        ├── patches/          # Resource modifications
        ├── namespace.yaml
        ├── network_policy.yaml
        ├── peer_authentication.yaml
        ├── sidecar.yaml
        ├── telemetry.yaml
        └── kustomization.yaml
```

### Key Design Patterns

1. **Dev Mode Configuration**: Vault runs with TLS disabled and file-based storage for development
2. **Auto-initialization**: `init.sh` automatically initializes and unseals Vault on startup
3. **StatefulSet Deployment**: Uses StatefulSet with persistent volume for data storage
4. **Istio Integration**: Development overlay includes service mesh sidecar injection
5. **Security Context**: Runs as non-root user (65532) with read-only root filesystem

### Configuration Details

**Vault Settings** (`config.hcl`):
- `disable_mlock = true` - Compatible with container environments
- `ui = false` - UI disabled for security
- TLS disabled (dev only)
- File storage at `/data`
- Listens on port 8200 (API) and 8201 (cluster)

**Auto-initialization** (`init.sh`):
- Checks if Vault is initialized
- If not, initializes and stores unseal keys and root token in `/data`
- Automatically unseals Vault using stored keys
- Runs as postStart lifecycle hook

**Security**:
- ServiceAccount with minimal permissions
- NetworkPolicy restricts access to ArgoCD repo-server only
- Non-privileged container with all capabilities dropped
- RuntimeDefault seccomp profile
- Read-only root filesystem with tmpfs mounts

## Important Notes

- This configuration is for **development use only** - production deployments should use proper TLS, external storage backend, and secure key management
- Unseal keys and root token are stored in the persistent volume (insecure for production)
- Vault is accessible only from ArgoCD namespace via NetworkPolicy
- Uses mirrored container image from `ghcr.io/kaidotio/hippocampus/mirror/vault`
- Development environment includes 1Gi persistent volume
- Single replica deployment (not HA)