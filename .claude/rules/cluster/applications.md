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

## Examples

Controllers and webhooks that expose a user-facing contract (annotation pattern, CR, resource convention) provide example manifests. Whether to create one or both directories depends on whether the skaffold overlay prefixes the annotation domain or CRD API group with `skaffold.`:

| User-facing contract? | skaffold prefix? | `examples/` | `skaffold/examples/` |
|-----------------------|-----------------|-------------|---------------------|
| Yes | Yes | Production-prefix examples | `skaffold.`-prefix examples |
| Yes | No | Examples (content would be identical) | Not needed |
| No (auto-intercepts all resources) | — | Optional (plain resource as test fixture) | Not needed |

| Location | Annotation/CRD prefix | Deployed automatically? |
|----------|----------------------|------------------------|
| `examples/` | Production (`app.kaidotio.github.io/...`) | No — reference documentation |
| `skaffold/examples/` | Dev (`skaffold.app.kaidotio.github.io/...`) | No — manually `kubectl apply` during `skaffold dev` |

Do NOT add example files to `skaffold/kustomization.yaml` resources — examples are applied manually, not auto-deployed.

## kind and e2e Tests

kind (isolated ephemeral cluster) is the primary decision. e2e tests follow from it — if kind is set up, write e2e to take advantage of the isolated cluster.

### kind

Use kind when the application needs an isolated cluster:

| Reason | kind.yaml needed? | Examples |
|--------|-------------------|----------|
| Benchmark accuracy (k6 latency needs no noisy neighbors) | No (default single-node) | envoy-markdownify, envoy-request-hasher, proxy-wasm |
| Node/cluster state pollution (privileged host mounts, cluster-scoped registrations) | Yes (if multi-node or extraMounts needed) | fuse-csi-driver |

Applications that interact only with K8s API (controllers, webhooks) or standard HTTP/gRPC (servers, CLI tools) do not need kind — unit tests with fake client or httptest suffice.

Makefile `e2e` target pattern: `kind create cluster` → `./e2e.sh` → kind is deleted by trap on exit.

### e2e

When kind is used, add e2e tests:

- `e2e.sh` — orchestration script (`skaffold run`, port-forward, run test client)
- `e2e/` — skaffold manifests for the e2e namespace
- Test client: `k6/` (load/functional), `playwright/` (browser), or `kubectl wait` (pod completion)

For docker-only runtimes (no Kubernetes needed), e2e uses docker directly without kind (e.g., chrome-devtools-protocol-server).

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
  Read: `.claude/reference/cluster/applications/controller.md`
If creating Webhook:
  Read: `.claude/reference/cluster/applications/webhook.md`
If creating Controller+Webhook:
  Read: `.claude/reference/cluster/applications/controller-webhook.md`
If creating Knative:
  Read: `.claude/reference/cluster/applications/knative.md`
If creating ext-proc:
  Read: `.claude/reference/cluster/applications/ext-proc.md`
