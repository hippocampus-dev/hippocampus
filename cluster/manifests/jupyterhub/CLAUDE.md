# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

JupyterHub is deployed as a Kubernetes-native multi-user notebook server environment with the following architecture:
- **Hub**: StatefulSet managing user sessions and authentication (using `jupyterhub-hub`)
- **Proxy**: Deployment routing traffic to user notebook pods (using `configurable-http-proxy`)
- **Singleuser Servers**: Dynamically spawned pods for individual user sessions

## Directory Structure

```
jupyterhub/
├── base/                    # Base Kustomize resources
│   ├── deployment.yaml      # Proxy deployment
│   ├── stateful_set.yaml    # Hub StatefulSet
│   ├── files/               # Configuration files
│   │   ├── jupyterhub_config.py  # Main JupyterHub configuration
│   │   ├── pyproject.toml        # Python dependencies
│   │   └── poetry.lock
│   └── kustomization.yaml   # Base Kustomize configuration
└── overlays/
    └── dev/                 # Development overlay
        ├── patches/         # Resource patches
        ├── redis/           # Redis backend configuration (optional)
        └── kustomization.yaml
```

## Common Development Commands

### Building Images
```bash
# Build JupyterHub hub image
cd /opt/hippocampus/cluster/applications/jupyterhub
docker build -t ghcr.io/kaidotio/hippocampus/jupyterhub .

# Build single-user notebook image
cd /opt/hippocampus/cluster/applications/singleuser-notebook
docker build -t ghcr.io/kaidotio/hippocampus/singleuser-notebook .

# Build configurable-http-proxy image
cd /opt/hippocampus/cluster/applications/configurable-http-proxy
docker build -t ghcr.io/kaidotio/hippocampus/configurable-http-proxy .
```

### Deployment
```bash
# Deploy to development environment
kubectl apply -k overlays/dev/

# Update image digests in kustomization files after building
cd base/
kustomize edit set image ghcr.io/kaidotio/hippocampus/jupyterhub=ghcr.io/kaidotio/hippocampus/jupyterhub@sha256:<new-digest>

# Check deployment status
kubectl -n jupyterhub get pods
kubectl -n jupyterhub logs -f deployment/jupyterhub-proxy
kubectl -n jupyterhub logs -f statefulset/jupyterhub-hub
```

### Testing
```bash
# Port-forward for local testing
kubectl -n jupyterhub port-forward service/jupyterhub-proxy 8080:8080

# Access JupyterHub at http://localhost:8080

# Check proxy routes
kubectl -n jupyterhub exec deployment/jupyterhub-proxy -- configurable-http-proxy --show-routes

# Check user pods
kubectl -n jupyterhub get pods -l app.kubernetes.io/component=singleuser-server
```

### Debugging
```bash
# Check hub logs
kubectl -n jupyterhub logs -f statefulset/jupyterhub-hub

# Check proxy logs
kubectl -n jupyterhub logs -f deployment/jupyterhub-proxy

# Check events
kubectl -n jupyterhub get events --sort-by='.lastTimestamp'

# Describe pod for detailed status
kubectl -n jupyterhub describe pod <pod-name>

# Access hub shell for debugging
kubectl -n jupyterhub exec -it statefulset/jupyterhub-hub -- /bin/bash
```

## High-Level Architecture

### Key Components

#### JupyterHub Hub (StatefulSet)
- Manages user authentication and session orchestration
- Uses header-based authentication (`X-Auth-Request-User`)
- Spawns single-user notebook pods using KubeSpawner
- Runs idle culler service to clean up unused sessions

#### Configurable HTTP Proxy (Deployment)
- Routes traffic between hub and user pods
- Provides health endpoints for monitoring
- Can optionally use Redis backend for state persistence

#### Single-User Notebooks
- Dynamically spawned pods per user session
- Two profiles: Standard (CPU) and GPU notebooks
- Persistent storage mounted from host paths
- Istio sidecar injection enabled for observability

### Configuration Details

#### Authentication
- Custom `HeaderAuthenticator` uses `X-Auth-Request-User` header
- Admin users defined in `c.Authenticator.admin_users`
- Auto-login enabled for seamless integration

#### Storage Volumes
- `/data/jupyterhub/persistent/{user}` - User-specific persistent storage
- `/data/jupyterhub/shared` - Shared storage (read-only for non-admins)
- `/data/jupyterhub/.cache/torch` - PyTorch cache (read-only for non-admins)
- Memory-backed checkpoints directory for notebook autosave

#### Resource Limits
- CPU: 1-4 cores per user
- Memory: 256MB-32GB per user
- GPU: Optional 1 GPU for GPU profile
- Ephemeral storage: 1GB minimum

#### Network Configuration
- Istio integration with sidecar injection
- Gateway hosts: `notebook.minikube.127.0.0.1.nip.io`, `notebook.kaidotio.dev`
- Service mesh telemetry and tracing enabled

## Important Patterns

### Dynamic Configuration
- `DynamicKubeSpawner` class enables user-specific volume mount configurations
- Template variables: `{user}`, `{is_admin}`, `{is_not_admin}`
- Supports dynamic path and permission configuration based on user roles

### Security Features
- Read-only root filesystem for security
- Non-root user execution (UID 65532)
- Capabilities dropped, privilege escalation disabled
- Network policies and peer authentication configured

### High Availability
- Horizontal Pod Autoscaler for proxy and hub
- Pod Disruption Budgets configured
- Rolling updates with proper lifecycle hooks

## Key Files to Understand

1. **base/files/jupyterhub_config.py**: Main configuration file
   - Contains `HeaderAuthenticator` implementation
   - Defines `DynamicKubeSpawner` for dynamic volume mounts
   - Configures idle culler service
   - Sets resource profiles (CPU/GPU)

2. **base/stateful_set.yaml**: Hub deployment
   - Defines hub container configuration
   - Mounts config and secrets
   - Sets up persistent storage

3. **base/deployment.yaml**: Proxy deployment
   - Configures HTTP proxy
   - Sets up health checks
   - Defines resource limits

4. **overlays/dev/kustomization.yaml**: Development overlay
   - Applies patches for dev environment
   - Configures Istio integration
   - Sets up Vault secrets

## Secrets Management
- Uses HashiCorp Vault integration via `SecretsFromVault` CRD
- `CONFIGPROXY_AUTH_TOKEN` shared between hub and proxy
- Stored at `/kv/data/jupyterhub` in Vault

## Monitoring & Observability
- Prometheus metrics exposed
- Istio telemetry configuration
- Cilium L7 visibility enabled
- Hubble exporter integration

## Troubleshooting

### Common Issues
1. **Pod spawn failures**: Check resource quotas and node capacity
2. **Authentication issues**: Verify header authentication is properly configured
3. **Storage permissions**: Ensure host paths have correct permissions
4. **Idle culler**: Adjust timeout values if notebooks are terminated too quickly

### Debug Commands
```bash
# Check hub logs
kubectl -n jupyterhub logs -f statefulset/jupyterhub-hub

# Check proxy routes
kubectl -n jupyterhub exec deployment/jupyterhub-proxy -- configurable-http-proxy --show-routes

# Check user pod status
kubectl -n jupyterhub get pods -l app.kubernetes.io/component=singleuser-server
```

## Notes
- The deployment supports both Istio proxy and configurable-http-proxy backends
- Redis backend is optional but provides better scalability
- Named servers feature allows users to run multiple notebook instances
- Idle culler runs every 10 minutes with 1-hour timeout

## Container Image Dependencies
- Hub image built from Poetry dependencies in `/opt/hippocampus/cluster/applications/jupyterhub/`
- Proxy image extends `quay.io/jupyterhub/configurable-http-proxy` with Redis backend support
- Singleuser notebook image based on `quay.io/jupyter/scipy-notebook` with additional packages