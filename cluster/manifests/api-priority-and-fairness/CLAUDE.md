# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

This directory contains Kubernetes API Priority and Fairness configuration manifests that control request prioritization for the Kubernetes API server. It uses Kustomize for manifest generation and is deployed via ArgoCD as part of the Hippocampus platform.

## Common Development Commands

### Building Manifests
```bash
# Build the dev overlay to see the generated manifests
kustomize build overlays/dev/

# Build the base configuration only
kustomize build base/
```

### Validation
```bash
# Validate the generated manifests
kustomize build overlays/dev/ | kubectl apply --dry-run=client -f -
```

## Architecture

### Directory Structure
- **`base/`** - Core FlowSchema definitions
  - `flow_schemas.yaml` - Defines API request prioritization rules
  - `kustomization.yaml` - Base Kustomize configuration
  
- **`overlays/dev/`** - Development environment overlay
  - `kustomization.yaml` - Adds namespace and references base

### FlowSchema Definitions
1. **health-for-strangers** - Allows unauthenticated access to health endpoints (/healthz, /livez, /readyz) with exempt priority
2. **vpa** - Grants high priority to Vertical Pod Autoscaler service accounts for all API operations

### Deployment Model
- Deployed via ArgoCD application defined in `/opt/hippocampus/cluster/manifests/argocd-applications/`
- Automatically synced on Git commits
- Deployed to `kube-system` namespace
- Uses GitOps workflow - changes should be committed to Git, not applied directly

## Development Workflow

1. Make changes to YAML files in `base/` or create new overlays
2. Test locally with `kustomize build`
3. Validate syntax with dry-run
4. Commit changes - ArgoCD will automatically deploy