# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This directory contains Kubernetes manifests for the ArgoCD Notifications Controller, which monitors ArgoCD applications and sends notifications to Slack when various events occur (deployments, failures, health degradation, etc.). It follows the Kustomize base/overlays pattern for environment-specific configuration.

## Common Development Commands

### Working with the Notifications Controller

```bash
# Apply the notifications controller to the dev environment
kubectl apply -k overlays/dev/

# Check the deployment status
kubectl get deployment -n argocd argocd-notifications-controller
kubectl get pods -n argocd -l app.kubernetes.io/name=argocd-notifications-controller

# View controller logs
kubectl logs -n argocd -l app.kubernetes.io/name=argocd-notifications-controller

# Check the ConfigMap with notification templates
kubectl describe configmap -n argocd argocd-notifications-cm

# Verify the secret is created from Vault
kubectl get secret -n argocd argocd-notifications-secret

# Test notification templates (dry run)
kubectl exec -n argocd deployment/argocd-notifications-controller -- argocd-notifications template notify --config-map argocd-notifications-cm --trigger on-deployed
```

### Modifying Notification Templates

When modifying notification templates in `overlays/dev/patches/config_map.yaml`:
- Templates use Go template syntax
- Available context variables: `{{.app}}`, `{{.context}}`, `{{.serviceType}}`, `{{.recipient}}`
- Slack attachments support markdown formatting
- Color codes: good (green), warning (yellow), danger (red)
- Test template changes before applying to production

### Adding New Notification Triggers

1. Add the trigger definition in `config_map.yaml` under `data.trigger.<trigger-name>`
2. Define the condition using Lua expressions
3. Add corresponding template in `data.template.<template-name>`
4. Reference the template in the trigger's `send` field

## High-Level Architecture

### Directory Structure
```
argocd-notifications-controller/
├── base/
│   ├── kustomization.yaml       # References upstream v2.14.15 + PodDisruptionBudget
│   └── pod_disruption_budget.yaml
└── overlays/
    └── dev/
        ├── kustomization.yaml   # Environment-specific configuration
        ├── patches/             # Kustomize patches for base resources
        │   ├── config_map.yaml  # Notification templates and triggers
        │   ├── deployment.yaml  # Production-ready deployment settings
        │   ├── pod_disruption_budget.yaml
        │   └── service.yaml
        ├── peer_authentication.yaml  # Istio mTLS enforcement
        ├── secrets_from_vault.yaml   # Vault secret injection
        ├── sidecar.yaml             # Istio sidecar configuration
        └── telemetry.yaml           # Observability configuration
```

### Key Configuration Components

**Base Layer**:
- Minimal configuration referencing upstream ArgoCD manifests
- Basic PodDisruptionBudget for availability

**Dev Overlay**:
- **Notification Configuration**: Comprehensive Slack templates for all ArgoCD events
- **Security**: Istio mTLS, non-root user, read-only filesystem, dropped capabilities
- **High Availability**: Topology spread constraints, controlled rolling updates
- **Observability**: Prometheus metrics (port 9001), OpenTelemetry tracing, Istio telemetry
- **Secret Management**: Slack bot token injected from Vault using SecretsFromVault

### Integration Points

1. **ArgoCD**: Monitors Application resources in the cluster
2. **Slack**: Sends formatted notifications using bot token from Vault
3. **Istio**: Service mesh integration with mTLS and controlled egress
4. **Prometheus**: Exposes metrics on port 9001
5. **OpenTelemetry**: Sends traces to otel-agent

### Security Considerations

- Runs as non-root user (UID 65532)
- Read-only root filesystem with specific volume mounts for writable paths
- All capabilities dropped
- Seccomp profile enabled
- Istio sidecar with REGISTRY_ONLY egress mode
- Explicit egress allowlist for required services
