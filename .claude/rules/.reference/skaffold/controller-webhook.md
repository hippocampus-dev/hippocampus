# Controller+Webhook

Combined reconciliation loop and admission webhook.

## When to Use

- Controller that also needs admission control
- Watching resources AND mutating/validating on creation

## Example

MUST copy from: `cluster/applications/nodeport-controller/`

## Files

| Directory | File | Purpose |
|-----------|------|---------|
| manifests/ | kustomization.yaml | Image configuration |
| manifests/ | deployment.yaml | Controller + webhook server |
| manifests/ | service.yaml | Webhook endpoint |
| manifests/ | service_account.yaml | Pod identity |
| manifests/ | cluster_role.yaml | RBAC permissions |
| manifests/ | cluster_role_binding.yaml | RBAC binding |
| manifests/ | certificate.yaml | TLS certificate |
| manifests/ | issuer.yaml | cert-manager issuer |
| manifests/ | mutating_webhook_configuration.yaml | Webhook registration |
| manifests/ | pod_disruption_budget.yaml | Availability during updates |
| skaffold/ | kustomization.yaml | Development overlay |
| skaffold/ | namespace.yaml | Development namespace |
| skaffold/patches/ | certificate.yaml | Development certificate |
| skaffold/patches/ | deployment.yaml | Development overrides |
| skaffold/patches/ | mutating_webhook_configuration.yaml | Development webhook config |

## Key Modifications

- `manifests/kustomization.yaml`: Update image name and digest
- `manifests/cluster_role.yaml`: Update resource permissions
- `manifests/mutating_webhook_configuration.yaml`: Update webhook rules
- `skaffold/kustomization.yaml`: Set namespace, add `- path: patches/deployment.yaml` to patches
- `skaffold/namespace.yaml`: Set namespace name
- `skaffold/patches/deployment.yaml`: Update deployment name and container name to match the application
- `skaffold/patches/certificate.yaml`: Update certificate name and namespace
- `skaffold/patches/mutating_webhook_configuration.yaml`: Update webhook name and namespace
