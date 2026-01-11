---
paths:
  - "cluster/manifests/argocd-applications/**/*.yaml"
---

* Copy existing Application file (e.g., `bakery.yaml`) as template
* Update `metadata.name`, `spec.source.path`, `spec.destination.namespace`
* Add to `kustomization.yaml` in alphabetical order
* Set `manifest-generate-paths` annotation based on source directories

## Manifest Generate Paths

| Scenario | Annotation Value |
|----------|-----------------|
| Manifests only | `/cluster/manifests/{app-name}` |
| With application code | `/cluster/applications/{app-name};/cluster/manifests/{app-name}` |
