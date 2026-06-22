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
        - .spec.template.spec.containers[]?.readinessProbe
        - .spec.traffic
```

| Field | Reason |
|-------|--------|
| `.spec.template.spec.containers[]?.readinessProbe` | Knative controller injects default readiness probe |
| `.spec.traffic` | Knative controller manages traffic routing to revisions |

## Webhook caBundle ignoreDifferences

ArgoCD Applications that manage MutatingWebhookConfiguration or ValidatingWebhookConfiguration resources with cert-manager must include `ignoreDifferences` to prevent permanent OutOfSync caused by cert-manager injecting `caBundle`:

```yaml
spec:
  ignoreDifferences:
    - group: admissionregistration.k8s.io
      kind: MutatingWebhookConfiguration
      jqPathExpressions:
        - .webhooks[]?.clientConfig?.caBundle
```

| Kind | When to Include |
|------|-----------------|
| `MutatingWebhookConfiguration` | Application has `mutating_webhook_configuration.yaml` with cert-manager annotation |
| `ValidatingWebhookConfiguration` | Application has `validating_webhook_configuration.yaml` with cert-manager annotation |

## resourceFieldRef Divisor ignoreDifferences

ArgoCD Applications with `ServerSideApply=true` that manage Deployments or StatefulSets with containers using `resourceFieldRef` (e.g., `GOMAXPROCS`/`GOMEMLIMIT`) must include `ignoreDifferences` to prevent permanent OutOfSync caused by the Kubernetes API server normalizing the omitted `divisor` to `"0"` in live state. Client-side apply does not exhibit this drift because its 3-way merge ignores fields absent from `last-applied-configuration`.

```yaml
spec:
  ignoreDifferences:
    - group: apps
      kind: Deployment
      jqPathExpressions:
        - .spec.template.spec.containers[].env[]?.valueFrom?.resourceFieldRef?.divisor?
    - group: apps
      kind: StatefulSet
      jqPathExpressions:
        - .spec.template.spec.containers[].env[]?.valueFrom?.resourceFieldRef?.divisor?
```

Use `?` (null-safe) operators on array iterations and field accesses. Without them, jq fails with "Cannot iterate over null" when any container lacks an `env` field, silently disabling the entire ignoreDifferences normalization.

| Kind | When to Include |
|------|-----------------|
| `Deployment` | Application has `ServerSideApply=true` and containers using `resourceFieldRef` (e.g., Go workloads with `GOMAXPROCS`/`GOMEMLIMIT`) |
| `StatefulSet` | Application has `ServerSideApply=true` and containers using `resourceFieldRef` |

## StatefulSet volumeClaimTemplates ignoreDifferences

ArgoCD Applications with `ServerSideApply=true` that manage StatefulSets with `volumeClaimTemplates` must include `ignoreDifferences` to prevent permanent OutOfSync caused by the Kubernetes API server injecting fields into the live state. Client-side apply does not exhibit this drift because its 3-way merge ignores fields absent from `last-applied-configuration`.

```yaml
spec:
  ignoreDifferences:
    - group: apps
      kind: StatefulSet
      jqPathExpressions:
        - .spec.volumeClaimTemplates[].apiVersion
        - .spec.volumeClaimTemplates[].kind
        - .spec.volumeClaimTemplates[].spec.volumeMode
```

| Field | Reason |
|-------|--------|
| `.spec.volumeClaimTemplates[].apiVersion` | API server injects `v1` when omitted in manifest |
| `.spec.volumeClaimTemplates[].kind` | API server injects `PersistentVolumeClaim` when omitted in manifest |
| `.spec.volumeClaimTemplates[].spec.volumeMode` | API server injects `Filesystem` when omitted in manifest |

If the application also has `resourceFieldRef` or `rollingUpdate.partition` drift, merge all `jqPathExpressions` lists under a single `StatefulSet` entry.

## StatefulSet rollingUpdate.partition ignoreDifferences

ArgoCD Applications with `ServerSideApply=true` that manage StatefulSets must include `ignoreDifferences` for `.spec.updateStrategy.rollingUpdate.partition` to prevent permanent OutOfSync caused by the Kubernetes API server defaulting this field to `0` in the live state when it is absent from the kustomize-generated manifest. Client-side apply does not exhibit this drift because its 3-way merge ignores fields absent from `last-applied-configuration`.

```yaml
spec:
  ignoreDifferences:
    - group: apps
      kind: StatefulSet
      jqPathExpressions:
        - .spec.updateStrategy.rollingUpdate.partition
```

| Field | Reason |
|-------|--------|
| `.spec.updateStrategy.rollingUpdate.partition` | API server defaults to `0` when omitted in manifest |

If the application also has `resourceFieldRef` or `volumeClaimTemplates` drift, merge all `jqPathExpressions` lists under a single `StatefulSet` entry.
