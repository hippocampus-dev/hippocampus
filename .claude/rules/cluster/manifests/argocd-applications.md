---
paths:
  - "cluster/manifests/argocd-applications/**/*.yaml"
---

* Copy existing Application file (e.g., `bakery.yaml`) as template
* Update `metadata.name`, `spec.source.path`, `spec.destination.namespace`
* Add to `kustomization.yaml` in alphabetical order
* Set `manifest-generate-paths` annotation to list all directories in the kustomize dependency tree (walk `resources`, `components`, `patches`, generators transitively from `spec.source.path`)
* Derive `argocd.argoproj.io/sync-wave` from the Applications this one cannot apply without, leaving it `"0"` where nothing else has to exist first (see Sync Wave below)

## Manifest Generate Paths

| Scenario | Annotation Value |
|----------|-----------------|
| Manifests in `cluster/manifests/{app-name}/` only | `/cluster/manifests/{app-name}` |
| Kustomize resources reference `cluster/applications/{app-name}/manifests/` | `/cluster/applications/{app-name};/cluster/manifests/{app-name}` |
| Kustomize resources reference `cluster/manifests/utilities/{utility}/` | Append `;/cluster/manifests/utilities/{utility}` for each utility |
| Sub-component references `cluster/applications/{component}/manifests/` | Append `;/cluster/applications/{component}` for each sub-component |

## Sync Wave

An Application's `argocd.argoproj.io/sync-wave` orders the app-of-apps sync alone, on its own scale, separate from the resource-level waves `.claude/rules/cluster/manifests.md` governs under its `## ArgoCD Sync Waves`.
Name the Application depended on in a trailing comment as `"{wave}" # {dependency} + {N}`, since the number alone does not say what it is waiting for and nothing rechecks it.

| Depends on | Wave |
|------------|------|
| A namespace another Application's overlay defines in `namespace.yaml` | One wave behind that Application |
| A custom resource whose CustomResourceDefinition another Application installs | One wave behind that Application |
| A namespace a `cluster/bin/deploy-*.sh` creates before the app-of-apps Application exists | `"0"` |
| Nothing | `"0"` |

Take where the resources land from the overlay's `kustomization.yaml` `namespace:` field rather than from `spec.destination.namespace`, which names a different namespace on the Applications that share one, and which `CreateNamespace=false` leaves ArgoCD creating in neither case.
`.github/workflows/50_sync-wave-validation.yaml` derives these dependencies from the built manifests, so it reports a missing wave only where the dependency is a namespace or a custom resource - one carried in a ConfigMap's string body or in a controller's readiness stays invisible to it and has to be declared by hand.
A wave that is too low fails the sync with `namespaces "{name}" not found` or an unknown kind, and ArgoCD never retries a failed revision, so the Application stays `OutOfSync`/`Missing` until git moves rather than recovering on the next reconcile.

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

ArgoCD Applications with `ServerSideApply=true` that manage Deployments or StatefulSets with containers using `resourceFieldRef` (e.g., `GOMAXPROCS`/`GOMEMLIMIT`) must include `ignoreDifferences` to prevent permanent OutOfSync caused by the Kubernetes API server normalizing the omitted `divisor` to `"0"` in live state.
Client-side apply does not exhibit this drift because its 3-way merge ignores fields absent from `last-applied-configuration`.

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

Use `?` (null-safe) operators on array iterations and field accesses.
Without them, jq fails with "Cannot iterate over null" when any container lacks an `env` field, silently disabling the entire ignoreDifferences normalization.

| Kind | When to Include |
|------|-----------------|
| `Deployment` | Application has `ServerSideApply=true` and containers using `resourceFieldRef` (e.g., Go workloads with `GOMAXPROCS`/`GOMEMLIMIT`) |
| `StatefulSet` | Application has `ServerSideApply=true` and containers using `resourceFieldRef` |

## StatefulSet volumeClaimTemplates ignoreDifferences

ArgoCD Applications with `ServerSideApply=true` that manage StatefulSets with `volumeClaimTemplates` must include `ignoreDifferences` to prevent permanent OutOfSync caused by the Kubernetes API server injecting fields into the live state.
Client-side apply does not exhibit this drift because its 3-way merge ignores fields absent from `last-applied-configuration`.

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

ArgoCD Applications with `ServerSideApply=true` that manage StatefulSets must include `ignoreDifferences` for `.spec.updateStrategy.rollingUpdate.partition` to prevent permanent OutOfSync caused by the Kubernetes API server defaulting this field to `0` in the live state when it is absent from the kustomize-generated manifest.
Client-side apply does not exhibit this drift because its 3-way merge ignores fields absent from `last-applied-configuration`.

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

## Deployment status.terminatingReplicas ignoreDifferences

ArgoCD Applications with `ServerSideApply=true` that manage Deployments must include `ignoreDifferences` for `.status.terminatingReplicas` to prevent the Application getting stuck at sync status `Unknown` with a `ComparisonError` condition reading `error building typed value from live resource: .status.terminatingReplicas: field not declared in schema`.
The Kubernetes API server populates this field on every Deployment (`kubectl explain deployment.status.terminatingReplicas` - a beta field, not present on StatefulSet's `.status`); ArgoCD's structured merge diff (used only under `ServerSideApply=true`) fails to type the whole live object once it hits a field absent from its own bundled schema, aborting the comparison rather than reporting a diff.
Client-side apply does not exhibit this because its 3-way merge never types the live object against a schema.

```yaml
spec:
  ignoreDifferences:
    - group: apps
      kind: Deployment
      jqPathExpressions:
        - .status.terminatingReplicas?
```

| Field | Reason |
|-------|--------|
| `.status.terminatingReplicas` | API server populates a field absent from ArgoCD's bundled schema |

If the application also has `resourceFieldRef` drift, merge both `jqPathExpressions` lists under a single `Deployment` entry.
