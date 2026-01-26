---
paths:
  - "cluster/applications/**"
---

* Source code, Dockerfile, Makefile for applications built in this repository
* Base Kubernetes manifests in `manifests/`, local development overlay in `skaffold/`
* Each application deployed to cluster also requires `cluster/manifests/{app}/` for environment overlays

## Directory Structure

Applications with source code require files in TWO locations:

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

## Workflow

1. Find a similar existing application - copy from BOTH `cluster/applications/{app}/` AND `cluster/manifests/{app}/`

2. In `cluster/applications/{app-name}/`:
   - Source code and Dockerfile
   - `manifests/`: base Kubernetes manifests
   - `skaffold/`: local development overlay (namespace: `skaffold-{app-name}`)
   - `skaffold.yaml`: Skaffold configuration

3. In `cluster/manifests/{app-name}/`:
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
