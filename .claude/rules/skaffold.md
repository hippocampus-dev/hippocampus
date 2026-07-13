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

## OpenTelemetry Trace Export

When an application initializes an OTLP trace exporter (e.g., Go `otlptracegrpc.New`, Python `OTLPSpanExporter`), the skaffold overlay does not provide the in-cluster `otel-agent` endpoint that production overlays set via `OTEL_EXPORTER_OTLP_ENDPOINT`. Disable trace export in the skaffold patch to avoid `localhost:4317` connection-refused noise during `make dev`:

| Application emits OTLP traces? | skaffold patch env |
|--------------------------------|--------------------|
| Yes | Add `OTEL_TRACES_SAMPLER=always_off` |
| No | Omit |

Do NOT set `OTEL_EXPORTER_OTLP_ENDPOINT` in skaffold patches — `always_off` short-circuits before the exporter dials.

Example: `cluster/applications/kube-crud-server/skaffold/patches/deployment.yaml`

## Reference

For application directory structure, workflow, and patterns:
  Read: `.claude/rules/cluster/applications.md`
