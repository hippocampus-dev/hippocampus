---
description: Create a new ArgoCD Application
argument-hint: [APPLICATION_NAME]
---

Follow these steps to create a new ArgoCD Application.

1. Create a new ArgoCD Application manifest in `cluster/manifests/argocd-applications/base/$ARGUMENT.yaml`
2. Modify `spec.destination.namespace` to match the namespace of the application
3. If `cluster/manifests/$ARGUMENT/base/kustomization.yaml` references `cluster/applications/$ARGUMENT`, include `/cluster/applications/$ARGUMENT` in the `argocd.argoproj.io/manifest-generate-paths` annotation
4. Add a new entry to `cluster/manifests/argocd-applications/base/kustomization.yaml`
