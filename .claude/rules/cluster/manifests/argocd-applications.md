---
paths:
  - "cluster/manifests/argocd-applications/**/*.yaml"
---

* Copy existing Application file (e.g., `bakery.yaml`) as template
* Update `metadata.name`, `spec.source.path`, `spec.destination.namespace`
* Add to `kustomization.yaml` in alphabetical order
* Set `manifest-generate-paths` annotation to list all directories in the kustomize dependency tree (walk `resources`, `components`, `patches`, generators transitively from `spec.source.path`)

## Manifest Generate Paths

| Scenario | Annotation Value |
|----------|-----------------|
| Manifests in `cluster/manifests/{app-name}/` only | `/cluster/manifests/{app-name}` |
| Kustomize resources reference `cluster/applications/{app-name}/manifests/` | `/cluster/applications/{app-name};/cluster/manifests/{app-name}` |
| Kustomize resources reference `cluster/manifests/utilities/{utility}/` | Append `;/cluster/manifests/utilities/{utility}` for each utility |
| Sub-component references `cluster/applications/{component}/manifests/` | Append `;/cluster/applications/{component}` for each sub-component |
