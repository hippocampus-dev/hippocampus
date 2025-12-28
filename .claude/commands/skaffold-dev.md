---
description: Create manifests and skaffold directories for local Kubernetes development
argument-hint: [APPLICATION_NAME]
---

Follow these steps to create manifests and skaffold directories for $ARGUMENT.

1. Find a similar existing application in `cluster/applications/` that has `manifests/` and `skaffold/` directories
2. Create `cluster/applications/$ARGUMENT/manifests/` with:
   - `kustomization.yaml`
   - Kubernetes manifests required for the application (e.g., `deployment.yaml`, `service.yaml`)
3. Create `cluster/applications/$ARGUMENT/skaffold/` with:
   - `kustomization.yaml` referencing `../manifests` as base
   - `namespace.yaml`
   - `patches/` directory for local development overrides
4. Create `cluster/applications/$ARGUMENT/skaffold.yaml`
5. Add exclusions for `manifests/` and `skaffold/` to `.github/workflows/00_$ARGUMENT.yaml` if it exists
