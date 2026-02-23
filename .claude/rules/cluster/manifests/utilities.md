---
paths:
  - "cluster/manifests/utilities/**/*.yaml"
---

* Development environment utilities (redis, minio, memcached, etc.) referenced by other manifests
* Flat structure (no base/overlays)
* Referenced from other manifests as `overlays/dev/redis/`, `overlays/dev/minio/`, etc.
* When modifying utilities, find and update all consumers: `grep -r "utilities/{name}" cluster/manifests/**/kustomization.yaml`

## Labels

Utility base manifests use only `app.kubernetes.io/component` in selectors. Do NOT add `app.kubernetes.io/name` — it is injected by consumer overlays via `includeSelectors: true`.

| Label | Defined in | Example |
|-------|-----------|---------|
| `app.kubernetes.io/component` | Utility base manifest | `http-redis-proxy` |
| `app.kubernetes.io/name` | Consumer overlay (`includeSelectors: true`) | `argocd` |

## Workload Type Selection

| Workload | When to Use | Example Template |
|----------|-------------|------------------|
| StatefulSet | Persistent storage, stable network identity | `cluster/manifests/utilities/redis/` |
| Deployment | Stateless, no persistent storage needed | `cluster/manifests/utilities/httpbin/` |
| Knative Service | Serverless, scale-to-zero, event sink/relay | `cluster/manifests/utilities/cloudevents-relay/` |

## Referencing Utilities

When referencing utilities from `overlays/dev/{utility}/kustomization.yaml`:

```yaml
labels:
  - includeSelectors: true
    pairs:
      app.kubernetes.io/name: {parent-app}
      variant: utilities
```

* Always add `variant: utilities` label to distinguish utility resources from main application

### Knative Service Utilities

Knative Service utilities MUST NOT use `includeSelectors: true`. Kustomize adds `spec.selector` to Knative Services (confusing `serving.knative.dev/v1 Service` with `v1 Service`), which is an invalid field and breaks the resource.

```yaml
labels:
  - pairs:
      app.kubernetes.io/name: {parent-app}
      variant: utilities
```

| Utility Workload Type | `includeSelectors: true` | Reason |
|----------------------|--------------------------|--------|
| Deployment / StatefulSet | Required | Injects `app.kubernetes.io/name` into pod selectors |
| Knative Service | Must NOT use | Adds invalid `spec.selector` to Knative Service |

Consumer Istio resources (PeerAuthentication, Sidecar, Telemetry) use `app.kubernetes.io/component` in `workloadSelector`/`selector` (matching the utility base pod template labels), since `app.kubernetes.io/name` is not injected into pod template labels without `includeSelectors`.

When using `namePrefix` with a Gateway and VirtualService, the VirtualService `gateways` reference must use the full prefixed name (e.g., `memory-bank-cloudevents-ingress`). Kustomize `namePrefix` does not auto-update cross-resource references in VirtualService specs.

### Required Patches

Consumers MUST create `patches/` directory. Copy from existing consumer of the same utility type.

| Patch | When Required | Content |
|-------|---------------|---------|
| `patches/service.yaml` | Always | Deployment/StatefulSet: `trafficDistribution: PreferClose`; Knative: env overrides |
| `patches/pod_disruption_budget.yaml` | Deployment/StatefulSet only | `maxUnavailable: 1` |
| `patches/deployment.yaml` or `patches/stateful_set.yaml` | Deployment/StatefulSet only | Istio sidecar, topologySpread, resources, env vars |

Workload patches vary by utility (Istio annotations, zone spreading, sidecars). Find existing consumer of the same utility and copy its patches.

### NetworkPolicy Egress for Istio Sidecars

When a utility has `policyTypes: Egress` in its NetworkPolicy and Istio sidecar is enabled, cross-namespace egress rules (istio-system/istiod, otel/otel-agent) must be defined in the consumer's overlay, not in the utility.

Kustomize `labels includeSelectors: true` adds consumer labels to all `podSelector` fields including egress targets. This breaks cross-namespace egress because target pods do not have consumer-specific labels.

| Egress Target | Define In |
|---------------|-----------|
| Same namespace (peer pods, DNS) | Utility `network_policy.yaml` |
| Cross-namespace (istio-system, otel) | Consumer's `network_policy.yaml` |

Add to existing `network_policy.yaml` in consumer's overlay (append with `---` separator):

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: {parent-app}-{utility}-egress
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/component: {utility-component}
  policyTypes:
    - Egress
  egress:
    - to:
        - namespaceSelector:
            matchLabels:
              name: istio-system
          podSelector:
            matchLabels:
              app: istiod
      ports:
        - protocol: TCP
          port: 15012
    - to:
        - namespaceSelector:
            matchLabels:
              name: otel
          podSelector:
            matchLabels:
              app.kubernetes.io/name: otel-agent
              app.kubernetes.io/component: ""
      ports:
        - protocol: TCP
          port: 4317
```

Example: `cluster/manifests/argocd/overlays/dev/network_policy.yaml`

### Overriding Utility Configuration

When a utility has default configuration files in a ConfigMap (e.g., varnish's `default.vcl`), use `behavior: replace` to override:

```yaml
configMapGenerator:
  - files:
      - files/default.vcl
    name: varnish
    behavior: replace
    options:
      immutable: true
```

| Utility has ConfigMap | Action |
|-----------------------|--------|
| Yes (default config) | Use `behavior: replace` to override |
| No | Create new ConfigMap (no behavior needed) |

Example: `cluster/manifests/embedding-gateway/overlays/dev/varnish/kustomization.yaml`

## Files

| File | Purpose |
|------|---------|
| kustomization.yaml | Image configuration |
| deployment.yaml | Deployment workload (stateless) |
| stateful_set.yaml | StatefulSet workload (persistent storage) |
| service.yaml | ClusterIP, Headless, or Knative Service |
| kustomizeconfig.yaml | Kustomize Knative support (Knative Service only) |
| pod_disruption_budget.yaml | Availability during updates |
| files/ | Configuration files (optional) |
| horizontal_pod_autoscaler.yaml | HPA (optional) |
| network_policy.yaml | Network rules (optional) |
