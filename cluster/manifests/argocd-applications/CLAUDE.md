# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This directory contains ArgoCD Application manifests that define how the Hippocampus platform services are deployed to Kubernetes. Each YAML file in the `base/` directory represents an ArgoCD Application resource that manages the deployment of a specific component.

## Common Development Commands

### Working with ArgoCD Applications

```bash
# Apply a single ArgoCD application (from the base directory)
kubectl apply -f base/<application-name>.yaml

# Apply all applications using Kustomize (development environment)
kubectl apply -k overlays/dev/

# Check ArgoCD application status
kubectl get applications -n argocd
kubectl describe application <app-name> -n argocd

# Sync an application manually
kubectl patch application <app-name> -n argocd --type merge -p '{"operation": {"sync": {}}}'

# Delete an ArgoCD application (will cascade delete managed resources)
kubectl delete application <app-name> -n argocd
```

### Adding a New Application

1. Create a new YAML file in `base/` following the standard pattern
2. Add the filename to `base/kustomization.yaml` resources list
3. Ensure the corresponding manifests exist at `cluster/manifests/<service-name>/overlays/dev`

### Modifying Applications

When modifying ArgoCD applications:
- Changes to sync-wave affect deployment order (lower numbers deploy first)
- The path must match the actual manifest location in the repository
- namespace in destination should match the service's deployment namespace

## High-Level Architecture

### Directory Structure
```
argocd-applications/
├── base/                    # Core ArgoCD Application definitions
│   ├── *.yaml              # Individual application manifests (70+ services)
│   └── kustomization.yaml  # Lists all applications
└── overlays/
    └── dev/                # Development environment
        └── kustomization.yaml  # Sets namespace to 'argocd'
```

### ArgoCD Application Pattern

All applications follow this consistent structure:
- **API Version**: `argoproj.io/v1alpha1`
- **Sync Wave**: Controls deployment order (-50 to 0)
- **Notifications**: Slack alerts on sync failures
- **Source**: Points to `git@github.com:kaidotio/hippocampus` with path to service manifests
- **Sync Policy**: Automated with `prune: true` and `selfHeal: true`
- **Kustomize**: Uses environment variable substitution for annotations

### GitOps Workflow

1. ArgoCD applications monitor the Git repository
2. Changes to manifests in `cluster/manifests/<service>/` trigger automatic sync
3. ArgoCD applies changes to the cluster maintaining desired state
4. Failed syncs trigger Slack notifications

### Service Categories

The applications manage various service types:
- **Infrastructure**: cert-manager, prometheus, grafana, loki, mimir
- **Networking**: cilium, istio, gateway-api, envoy-ratelimit
- **AI/ML Services**: embedding-gateway, whisper-worker, translator
- **Developer Tools**: jupyterhub, github-actions-runner-controller
- **Platform Services**: Various pod hooks, controllers, and operators
- **Monitoring**: metrics-server, node-exporter, kube-state-metrics

Each service's actual Kubernetes manifests are located at `cluster/manifests/<service-name>/` with their own base/overlays structure.