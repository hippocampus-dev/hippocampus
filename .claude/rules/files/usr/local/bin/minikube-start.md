---
paths:
  - "files/usr/local/bin/minikube-start.sh"
---

* Find the manifests using an API before dropping its entry from `FEATURE_GATE_OPTION` or from `apiserver.runtime-config` - `admissionregistration.k8s.io/v1alpha1` is served only because this script names it and `cluster/manifests/mutating-admission/base/` is what depends on that, while `.github/workflows/50_kubectl-validation.yaml` carries no `paths` entry under `files/`, so the removal reaches no validation and surfaces only as that ArgoCD Application failing to sync once the cluster is next rebuilt
