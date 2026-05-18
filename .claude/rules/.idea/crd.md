---
paths:
  - ".idea/crd/**"
---

* `kustomization.yaml` to modify CRD sources; `crd.yaml` is the generated output
* After modifying `kustomization.yaml`, regenerate `crd.yaml` with `kustomize build .idea/crd/ > .idea/crd/crd.yaml`
