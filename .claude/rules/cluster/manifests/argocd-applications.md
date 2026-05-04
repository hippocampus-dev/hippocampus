---
paths:
  - "cluster/manifests/argocd-applications/**/*.yaml"
---

* Copy existing Application file (e.g., `bakery.yaml`) as template
* Update `metadata.name`, `spec.source.path`, `spec.destination.namespace`
* Add to `kustomization.yaml` in alphabetical order
* Set `manifest-generate-paths` annotation to list all directories in the kustomize dependency tree (walk `resources`, `components`, `patches`, generators transitively from `spec.source.path`)

## Manifest Generate Paths

| Scenario | Annotation Value |
|----------|-----------------|
| Manifests in `cluster/manifests/{app-name}/` only | `/cluster/manifests/{app-name}` |
| Kustomize resources reference `cluster/applications/{app-name}/manifests/` | `/cluster/applications/{app-name};/cluster/manifests/{app-name}` |
| Kustomize resources reference `cluster/manifests/utilities/{utility}/` | Append `;/cluster/manifests/utilities/{utility}` for each utility |
| Sub-component references `cluster/applications/{component}/manifests/` | Append `;/cluster/applications/{component}` for each sub-component |

## Knative Service ignoreDifferences

ArgoCD Applications that manage Knative Services (`serving.knative.dev/Service`) must include `ignoreDifferences` to prevent permanent OutOfSync caused by Knative controller injecting default values:

```yaml
spec:
  ignoreDifferences:
    - group: serving.knative.dev
      kind: Service
      jqPathExpressions:
        - .spec.template.spec.containers[].readinessProbe
        - .spec.traffic
```

| Field | Reason |
|-------|--------|
| `.spec.template.spec.containers[].readinessProbe` | Knative controller injects default readiness probe |
| `.spec.traffic` | Knative controller manages traffic routing to revisions |

## Webhook caBundle ignoreDifferences

ArgoCD Applications that manage MutatingWebhookConfiguration or ValidatingWebhookConfiguration resources with cert-manager must include `ignoreDifferences` to prevent permanent OutOfSync caused by cert-manager injecting `caBundle`:

```yaml
spec:
  ignoreDifferences:
    - group: admissionregistration.k8s.io
      kind: MutatingWebhookConfiguration
      jqPathExpressions:
        - .webhooks[].clientConfig.caBundle
```

| Kind | When to Include |
|------|-----------------|
| `MutatingWebhookConfiguration` | Application has `mutating_webhook_configuration.yaml` with cert-manager annotation |
| `ValidatingWebhookConfiguration` | Application has `validating_webhook_configuration.yaml` with cert-manager annotation |
