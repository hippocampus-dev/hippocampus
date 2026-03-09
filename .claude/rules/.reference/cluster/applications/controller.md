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
| manifests/ | service.yaml | Application endpoints (optional; only for webhooks/APIs) |
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

## Custom Resource Placement

CRD definitions go in the controller directory, but CR instances go in the consumer directory:

| Resource | Location | Example |
|----------|----------|---------|
| CRD (CustomResourceDefinition) | `cluster/applications/{controller}/manifests/crd/` | `exporters.github-actions-exporter.kaidotio.github.io` |
| CR (Custom Resource instance) | `cluster/manifests/{consumer}/` | `cluster/manifests/runner/overlays/dev/exporter.yaml` |

The controller watches CRs cluster-wide; placing CRs in consumer directories keeps related resources together and follows namespace isolation patterns.

| Controller | CR Kind | Consumer Example |
|------------|---------|------------------|
| github-actions-runner-controller | Runner | `cluster/manifests/runner/` |
| github-actions-exporter-controller | Exporter | `cluster/manifests/runner/` |
| grafana-manifest-controller | Dashboard | `cluster/manifests/grafana/` |

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

3. `skaffold/kustomization.yaml`: Patch CRD name, group, and ClusterRole to use `skaffold.` prefix:
   ```yaml
   patches:
     - path: patches/deployment.yaml
     # CRD does not support namePrefix
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
     - patch: |
         - op: add
           path: /rules/0
           value:
             apiGroups:
               - skaffold.{app-name}.kaidotio.github.io
             resources:
               - "*"
             verbs:
               - "*"
       target:
         kind: ClusterRole
         name: {app-name}
   ```

| Patch | Purpose |
|-------|---------|
| CRD name/group | Allow skaffold and production CRDs to coexist |
| ClusterRole rules | Grant controller permissions for skaffold-prefixed API group |

The `variant: skaffold` label is already added by the skaffold kustomization.yaml `labels` block.
