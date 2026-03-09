# Webhook

Kubernetes admission webhook for mutating or validating resources.

## When to Use

- Mutating resources before creation/update
- Validating resources against policies
- Injecting sidecars, labels, or annotations

## Example

Copy from: `exactly-one-pod-hook`

## Files (cluster/applications/{app}/)

| Directory | File | Purpose |
|-----------|------|---------|
| manifests/ | kustomization.yaml | Image configuration |
| manifests/ | deployment.yaml | Webhook server |
| manifests/ | service.yaml | Webhook endpoint |
| manifests/ | service_account.yaml | Pod identity |
| manifests/ | certificate.yaml | TLS certificate |
| manifests/ | issuer.yaml | cert-manager issuer |
| manifests/ | mutating_webhook_configuration.yaml | Webhook registration |
| manifests/ | pod_disruption_budget.yaml | Availability during updates |
| manifests/ | cluster_role.yaml | RBAC (if using client.Get/List) |
| manifests/ | cluster_role_binding.yaml | RBAC binding (if using client.Get/List) |
| skaffold/ | kustomization.yaml | Development overlay |
| skaffold/ | namespace.yaml | Development namespace |
| skaffold/ | redis.yaml | Dependency service (if needed) |
| skaffold/ | etcd.yaml | Dependency service (if needed) |
| skaffold/patches/ | certificate.yaml | Development certificate |
| skaffold/patches/ | deployment.yaml | Development overrides |
| skaffold/patches/ | mutating_webhook_configuration.yaml | Development webhook config |

## Handler Pattern

Use `switch (pair{req.Kind, req.Operation})` pattern for request filtering:

```go
func (h *handler) Handle(ctx context.Context, req admission.Request) admission.Response {
    handlerLogger := ctrl.Log.WithName("handler")

    type pair struct {
        gvk       metav1.GroupVersionKind
        operation admissionv1.Operation
    }
    switch (pair{req.Kind, req.Operation}) {
    case pair{metav1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "StatefulSet"}, admissionv1.Create}:
        // handle StatefulSet CREATE
    }

    return admission.Allowed("")
}
```

| Pattern | When to Use |
|---------|-------------|
| `switch (pair{req.Kind, req.Operation})` | Single resource type with specific operation |
| `switch req.Kind` | Single resource type with multiple operations |

Do NOT use `if req.Kind.Group != "..." || ...` pattern.

## RBAC for Kubernetes API Access

When a webhook uses `client.Get` or `client.List` to read Kubernetes resources, add RBAC:

| Webhook uses | Requires RBAC |
|--------------|---------------|
| Only `req.Object` (incoming request) | No |
| `client.Get` or `client.List` | Yes |

Example: `cluster/applications/statefulset-hook/manifests/cluster_role.yaml` (lists pods to count replicas)

## Key Modifications

- `manifests/kustomization.yaml`: Update image name and digest
- `manifests/mutating_webhook_configuration.yaml`: Update webhook rules
- `skaffold/kustomization.yaml`: Set namespace, add `- path: patches/deployment.yaml` to patches
- `skaffold/namespace.yaml`: Set namespace name
- `skaffold/patches/deployment.yaml`: Update deployment name and container name to match the application
- `skaffold/patches/certificate.yaml`: Update certificate name and namespace
- `skaffold/patches/mutating_webhook_configuration.yaml`: Update webhook name and namespace

## VARIANT Environment Variable

Webhooks that use custom annotation groups need the VARIANT environment variable to prefix annotation groups during Skaffold development. This allows development and production webhooks to coexist without conflicts.

**Required changes:**

1. Add `apiGroup()` function that reads VARIANT:
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

The `variant: skaffold` label is already added by the skaffold kustomization.yaml `labels` block.
