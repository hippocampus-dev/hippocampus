---
paths:
  - "cluster/applications/**"
---

* Source code, Dockerfile, Makefile for applications built in this repository

## Pattern Selection

Whether `manifests/` and `skaffold/` belong in `cluster/applications/{app}/` depends on whether the application requires Kubernetes to develop:

| Development requires Kubernetes? | `manifests/` + `skaffold/` | `dev` target | Example |
|----------------------------------|----------------------------|--------------|---------|
| Yes (webhooks, controllers) | In `cluster/applications/{app}/` | `skaffold dev --port-forward` | `exactly-one-pod-hook`, `nodeport-controller` |
| No (HTTP servers, CLI tools) | **Do not create** | `watchexec ... go run` | `bakery`, `github-token-server`, `http-redis-proxy` |

Applications that can run without Kubernetes have their manifests ONLY in `cluster/manifests/` (either `cluster/manifests/{app}/` or `cluster/manifests/utilities/{app}/`). Do NOT add `manifests/`, `skaffold/`, or `skaffold.yaml` to `cluster/applications/{app}/` for these.

## Directory Structure (Kubernetes-required applications)

Applications that require Kubernetes need files in TWO locations:

| Location | Purpose |
|----------|---------|
| `cluster/applications/{app}/` | Source code, Dockerfile, base manifests, local dev overlay |
| `cluster/manifests/{app}/` | Environment overlays (Istio, NetworkPolicy) for ArgoCD |

```
cluster/applications/{app}/
├── main.go (or main.py, etc.), Dockerfile, Makefile
├── manifests/           # Base Kubernetes manifests
├── skaffold/            # Local development overlay (namespace: skaffold-{app})
└── skaffold.yaml

cluster/manifests/{app}/
├── base/
│   └── kustomization.yaml  # References ../../../applications/{app}/manifests
└── overlays/dev/
    ├── namespace.yaml, network_policy.yaml, peer_authentication.yaml
    ├── sidecar.yaml, telemetry.yaml
    └── patches/         # Environment-specific patches
```

ArgoCD Application points to: `cluster/manifests/{app}/overlays/dev`

## Directory Structure (standalone applications)

Applications that can run without Kubernetes:

```
cluster/applications/{app}/
├── main.go (or main.py, etc.), Dockerfile, Makefile
└── (NO manifests/, skaffold/, or skaffold.yaml)
```

Kubernetes manifests are managed separately in `cluster/manifests/{app}/` or `cluster/manifests/utilities/{app}/`.

## Workflow

1. Find a similar existing application - determine if it requires Kubernetes for development

2. In `cluster/applications/{app-name}/`:
   - Source code and Dockerfile
   - If Kubernetes-required: add `manifests/`, `skaffold/`, `skaffold.yaml`
   - If standalone: source code and Dockerfile only

3. If Kubernetes-required, in `cluster/manifests/{app-name}/`:
   - `base/kustomization.yaml`: reference `../../../applications/{app-name}/manifests`
   - `overlays/dev/`: namespace, network_policy, peer_authentication, sidecar, telemetry
   - `overlays/dev/patches/`: environment-specific patches

4. Create ArgoCD Application pointing to `cluster/manifests/{app-name}/overlays/dev`

5. Create GitHub Actions workflow `.github/workflows/00_{app-name}.yaml`

## Application Patterns

| Pattern | When to Use |
|---------|-------------|
| Controller | Reconciliation loop for CRD or built-in resources |
| Webhook | Admission control (mutating/validating) |
| Controller+Webhook | Both reconciliation and admission |
| Knative | Serverless event-driven services |
| ext-proc | Envoy External Processor gRPC service |

## Reference

If writing skaffold.yaml or skaffold/ overlay files:
  Read: `.claude/rules/skaffold.md`

If creating environment overlays in cluster/manifests/{app}/:
  Read: `.claude/rules/cluster/manifests.md`

If creating Controller:
  Read: `.claude/rules/.reference/cluster/applications/controller.md`
If creating Webhook:
  Read: `.claude/rules/.reference/cluster/applications/webhook.md`
If creating Controller+Webhook:
  Read: `.claude/rules/.reference/cluster/applications/controller-webhook.md`
If creating Knative:
  Read: `.claude/rules/.reference/cluster/applications/knative.md`
If creating ext-proc:
  Read: `.claude/rules/.reference/cluster/applications/ext-proc.md`
