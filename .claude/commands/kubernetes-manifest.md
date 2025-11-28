---
description: Write Kubernetes manifests
argument-hint: [GITHUB_URL]
---

Follow these steps to write Kubernetes Manifests.

1. Open $ARGUMENT to understand the overview
2. Search for and copy highly related existing Kubernetes Manifests from `cluster/manifests`
3. Modify the Kubernetes Manifest to meet requirements while maintaining the existing structure as much as possible
    - IMPORTANT: Only include fields and configurations that are explicitly required. Do not add optional fields or other non-essential configurations unless specifically requested
4. Docker Images must not be referenced directly; they must first be mirrored using `.github/workflows/99_mirroring.yaml`
5. If `kustomization.yaml` already exists, modify it to reference the added resources
6. If a new `kustomization.yaml` is added, create a corresponding file in `cluster/manifests/argocd-applications/base` and reference it from `cluster/manifests/argocd-applications/base/kustomization.yaml` to enable deployment with ArgoCD
7. Confirm compliance with all previous instructions
