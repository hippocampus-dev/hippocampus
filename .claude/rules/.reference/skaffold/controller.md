# Controller

Reconciliation loop for CRD or built-in Kubernetes resources.

## When to Use

- Custom Resource management with controller-runtime
- Watching and reacting to built-in resource changes

## Example

MUST copy from: `cluster/applications/github-actions-runner-controller/` (CRD) or `cluster/applications/events-logger/` (built-in)

## Files

| Directory | File | Purpose |
|-----------|------|---------|
| (root) | kind.yaml | Local Kind cluster config (CRD controllers only) |
| manifests/ | kustomization.yaml | Image configuration |
| manifests/ | deployment.yaml | Controller workload |
| manifests/ | service.yaml | Metrics/health endpoints |
| manifests/ | service_account.yaml | Pod identity |
| manifests/ | cluster_role.yaml | RBAC permissions |
| manifests/ | cluster_role_binding.yaml | RBAC binding |
| manifests/ | pod_disruption_budget.yaml | Availability during updates |
| manifests/crd/ | kustomization.yaml | CRD kustomization (if needed) |
| manifests/crd/ | *.yaml | Custom Resource Definition (if needed) |
| skaffold/ | kustomization.yaml | Development overlay |
| skaffold/ | namespace.yaml | Development namespace |
| skaffold/patches/ | deployment.yaml | Development overrides |

## Key Modifications

- `manifests/kustomization.yaml`: Update image name and digest
- `manifests/cluster_role.yaml`: Update resource permissions
- `skaffold/kustomization.yaml`: Set namespace, add CRD namePrefix patches if using CRD, add `- path: patches/deployment.yaml` to patches
- `skaffold/namespace.yaml`: Set namespace name
- `skaffold/patches/deployment.yaml`: Update deployment name and container name to match the application

## CRD Controllers

For controllers with Custom Resource Definitions, add `kind.yaml` and override `dev` target in Makefile:

```makefile
.PHONY: dev
dev:
	@kind create cluster --name {app-name} --config kind.yaml
	@trap 'kind delete cluster --name {app-name}' EXIT ERR INT; skaffold dev --port-forward
```
