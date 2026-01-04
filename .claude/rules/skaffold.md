---
paths:
  - "**/skaffold.yaml"
  - "**/skaffold/**/*.yaml"
---

* API version `skaffold/v4beta9`
* Use `inputDigest` tag policy for content-based tagging
* Enable BuildKit with `useBuildkit: true`

## Workflow

1. Find a similar existing application in `cluster/applications/` that has `manifests/` and `skaffold/` directories

2. Copy `manifests/` directory to `cluster/applications/{app-name}/manifests/` and replace:
   - `kustomization.yaml`: update `images` section with the application's image
   - Kubernetes manifests: modify as required (e.g., `deployment.yaml`, `service.yaml`)

3. Copy `skaffold/` directory to `cluster/applications/{app-name}/skaffold/` and replace:
   - `kustomization.yaml`: set `namespace` to `skaffold-{app-name}`
   - `namespace.yaml`: set `metadata.name` to `skaffold-{app-name}`
   - `patches/`: modify for local development overrides

4. Copy `skaffold.yaml` to `cluster/applications/{app-name}/skaffold.yaml` and replace:
   - `build.artifacts[].image`: set to the application's image

5. Add exclusions for `manifests/` and `skaffold/` to `.github/workflows/00_{app-name}.yaml` if it exists

## Patterns

| Pattern | When to Use |
|---------|-------------|
| Controller | Reconciliation loop for CRD or built-in resources |
| Webhook | Admission control (mutating/validating) |
| Controller+Webhook | Both reconciliation and admission |
| Knative | Serverless event-driven services |

## Reference

If creating Controller:
  Read: `.claude/rules/.reference/skaffold/controller.md`
If creating Webhook:
  Read: `.claude/rules/.reference/skaffold/webhook.md`
If creating Controller+Webhook:
  Read: `.claude/rules/.reference/skaffold/controller-webhook.md`
If creating Knative:
  Read: `.claude/rules/.reference/skaffold/knative.md`
