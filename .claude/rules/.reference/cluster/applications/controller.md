# Controller

Reconciliation loop for CRD or built-in Kubernetes resources.

## When to Use

- Custom Resource management with controller-runtime
- Watching and reacting to built-in resource changes

## Example

| Type | Copy from |
|------|-----------|
| CRD controller | `github-actions-runner-controller` |
| Built-in resource | `events-logger` |

## Files (cluster/applications/{app}/)

| Directory | File | Purpose |
|-----------|------|---------|
| (root) | kind.yaml | Local Kind cluster config (CRD controllers only) |
| manifests/ | kustomization.yaml | Image configuration |
| manifests/ | deployment.yaml | Controller workload |
| manifests/ | service.yaml | Metrics/health endpoints |
| manifests/ | service_account.yaml | Pod identity |
| manifests/ | cluster_role.yaml | RBAC permissions |
| manifests/ | cluster_role_binding.yaml | RBAC binding |
| manifests/ | role.yaml | Leader election RBAC |
| manifests/ | role_binding.yaml | Leader election RBAC binding |
| manifests/ | pod_disruption_budget.yaml | Availability during updates |
| manifests/crd/ | kustomization.yaml | CRD kustomization (if needed) |
| manifests/crd/ | *.yaml | Custom Resource Definition (if needed) |
| skaffold/ | kustomization.yaml | Development overlay |
| skaffold/ | namespace.yaml | Development namespace |
| skaffold/patches/ | deployment.yaml | Development overrides |

## Key Modifications

- `manifests/kustomization.yaml`: Update image name and digest
- `manifests/cluster_role.yaml`: Update resource permissions
- `manifests/role.yaml`: Update name and resourceNames
- `manifests/role_binding.yaml`: Update name and subjects
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

### VARIANT Environment Variable

CRD controllers need the VARIANT environment variable to prefix API groups during Skaffold development. This allows development and production CRDs to coexist without conflicts.

**Required changes:**

1. `api/v1/groupversion_info.go`: Add `apiGroup()` function that reads VARIANT:
   ```go
   func apiGroup() string {
       defaultGroup := "{app-name}.kaidotio.github.io"
       if v, ok := os.LookupEnv("VARIANT"); ok {
           return fmt.Sprintf("%s.%s", v, defaultGroup)
       }
       return defaultGroup
   }
   ```

2. `skaffold/patches/deployment.yaml`: Inject VARIANT from pod label via Downward API:
   ```yaml
   env:
     - name: VARIANT
       valueFrom:
         fieldRef:
           fieldPath: metadata.labels['variant']
   ```

3. `skaffold/kustomization.yaml`: Patch CRD name and group to use `skaffold.` prefix:
   ```yaml
   patches:
     - patch: |
         - op: replace
           path: /metadata/name
           value: {resources}.skaffold.{app-name}.kaidotio.github.io
         - op: replace
           path: /spec/group
           value: skaffold.{app-name}.kaidotio.github.io
       target:
         kind: CustomResourceDefinition
         name: {resources}.{app-name}.kaidotio.github.io
   ```

The `variant: skaffold` label is already added by the skaffold kustomization.yaml `labels` block.
