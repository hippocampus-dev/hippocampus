# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This directory contains Kubernetes manifests for deploying the cortex-bot, an enterprise-grade Slack bot powered by OpenAI that provides AI-powered conversational assistance. The manifests follow a Kustomize-based structure with base configurations and environment-specific overlays.

## Common Development Commands

### Manifest Operations
```bash
# Deploy to development environment
kubectl apply -k cluster/manifests/cortex-bot/overlays/dev

# Build manifests without applying (dry-run)
kubectl kustomize cluster/manifests/cortex-bot/overlays/dev

# Validate manifests
kubectl apply -k cluster/manifests/cortex-bot/overlays/dev --dry-run=client --validate

# Delete deployment
kubectl delete -k cluster/manifests/cortex-bot/overlays/dev
```

### Application Development
```bash
# Development with auto-reload (in /opt/hippocampus/cluster/applications/bot/)
make dev

# Install dependencies only
make install

# Run tests
uv run -- python -m unittest discover

# Update dependencies
uv lock
```

### Image Management
```bash
# Update image digest in base/kustomization.yaml
# Format: sha256:<new-digest>

# View current image
kubectl get deployment cortex-bot -n cortex-bot -o jsonpath='{.spec.template.spec.containers[0].image}'

# Watch for pod rollout
kubectl rollout status deployment/cortex-bot -n cortex-bot
```

## High-Level Architecture

### Manifest Structure
```
cortex-bot/
├── base/                        # Base manifests (shared across environments)
│   ├── deployment.yaml         # Core deployment with security contexts
│   ├── kustomization.yaml      # Base kustomization with image references
│   └── pod_disruption_budget.yaml
└── overlays/
    └── dev/                    # Development environment
        ├── deployment.yaml     # Dev-specific patches
        ├── kustomization.yaml  # Dev overlays and resources
        ├── namespace.yaml      
        ├── service.yaml        
        ├── destination_rule.yaml    # Istio traffic management
        ├── horizontal_pod_autoscaler.yaml
        ├── network_policy.yaml      # Egress rules
        ├── peer_authentication.yaml # mTLS config
        ├── service_entry.yaml       # External service access
        ├── telemetry.yaml          # Observability config
        ├── minio/             # S3-compatible storage
        ├── redis/             # Cache and rate limiting
        └── patches/           # Strategic merge patches
```

### Key Design Patterns

1. **Security-First Design**:
   - Non-root user (65532)
   - Read-only root filesystem
   - No Linux capabilities
   - Restricted security context

2. **Kustomize Layering**:
   - Base contains minimal, environment-agnostic configs
   - Overlays add environment-specific features
   - Strategic merge patches for fine-grained control

3. **Service Mesh Integration**:
   - Istio sidecars for mTLS
   - Traffic policies via DestinationRule
   - Telemetry and observability

4. **External Dependencies**:
   - Redis (rate limiting, caching)
   - MinIO (file storage)
   - Vault (secrets management)
   - External APIs (OpenAI, Slack, etc.)

### Critical Configuration Elements

**Volume Requirements**:
- `/home/nonroot/.config/matplotlib`
- `/home/nonroot/.cache/fontconfig`
- `/home/nonroot/.cache/huggingface`
- `/home/nonroot/.cache/matplotlib`
- `/tmp`

**Environment Variables**:
- `LOG_LEVEL`: "warning" (default)
- `WEB_CONCURRENCY`: "1" (OpenTelemetry limitation)
- Plus secrets from Vault

**Resource Management**:
- HPA for auto-scaling
- PodDisruptionBudget for availability
- Memory-based emptyDir volumes

## Important Notes

1. **Secrets**: Never store secrets in manifests. Use `secrets_from_vault.yaml`
2. **Image Updates**: Always use SHA256 digests, never tags
3. **Network Policies**: Carefully review egress rules before adding new external services
4. **Istio**: Ensure sidecar injection is enabled in namespace
5. **Development**: The bot application uses UV package manager (not pip)

## Integration Points

- **Application Source**: `/opt/hippocampus/cluster/applications/bot/`
- **Shared Libraries**: `/opt/hippocampus/cluster/applications/packages/cortex`
- **Container Registry**: `ghcr.io/kaidotio/hippocampus/cortex-bot`
- **CI/CD**: GitHub Actions builds and pushes images
- **Deployment**: ArgoCD monitors this repository for GitOps

## Troubleshooting Guide

**Pod Issues**:
```bash
kubectl describe pod -n cortex-bot -l app=cortex-bot
kubectl logs -n cortex-bot -l app=cortex-bot
kubectl logs -n cortex-bot -l app=cortex-bot -c istio-proxy  # Sidecar logs
```

**Common Problems**:
1. **OOMKilled**: Increase memory limits in deployment
2. **CrashLoopBackOff**: Check logs, verify volume mounts
3. **Connection Refused**: Verify network policies and service entries
4. **Slow Startup**: Check readiness probe timing
5. **Secrets Missing**: Ensure Vault integration is working