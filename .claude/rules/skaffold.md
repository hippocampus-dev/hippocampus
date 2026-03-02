---
paths:
  - "**/skaffold.yaml"
  - "**/skaffold/**/*.yaml"
---

* API version `skaffold/v4beta9`
* Use `inputDigest` tag policy for content-based tagging
* Enable BuildKit with `useBuildkit: true`

## skaffold.yaml Configuration

```yaml
apiVersion: skaffold/v4beta9
kind: Config
build:
  artifacts:
    - image: ghcr.io/hippocampus-dev/hippocampus/{app-name}
  tagPolicy:
    inputDigest: {}
  local:
    useBuildkit: true
manifests:
  kustomize:
    paths:
      - skaffold
deploy:
  kubectl: {}
```

## skaffold/ Directory

Local development overlay with namespace `skaffold-{app-name}`:

| File | Purpose |
|------|---------|
| `kustomization.yaml` | References `../manifests`, sets namespace and labels |
| `namespace.yaml` | Development namespace definition |
| `patches/*.yaml` | Development-specific overrides |

## Reference

For application directory structure, workflow, and patterns:
  Read: `.claude/rules/cluster/applications.md`
