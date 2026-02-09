---
paths:
  - "cluster/manifests/utilities/**/*.yaml"
---

* Development environment utilities (redis, minio, memcached, etc.) referenced by other manifests
* Flat structure (no base/overlays)
* Referenced from other manifests as `overlays/dev/redis/`, `overlays/dev/minio/`, etc.
* When modifying utilities, find and update all consumers: `grep -r "utilities/{name}" cluster/manifests/**/kustomization.yaml`

## Workload Type Selection

| Workload | When to Use | Example Template |
|----------|-------------|------------------|
| StatefulSet | Persistent storage, stable network identity | `cluster/manifests/utilities/redis/` |
| Deployment | Stateless, no persistent storage needed | `cluster/manifests/utilities/httpbin/` |

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

### Required Patches

Consumers MUST create `patches/` directory. Copy from existing consumer of the same utility type.

| Patch | Always Required | Content |
|-------|-----------------|---------|
| `patches/service.yaml` | Yes | `trafficDistribution: PreferClose` |
| `patches/pod_disruption_budget.yaml` | Yes | `maxUnavailable: 1` |
| `patches/deployment.yaml` or `patches/stateful_set.yaml` | Varies | Istio sidecar, topologySpread, resources, env vars |

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
| service.yaml | ClusterIP or Headless service |
| pod_disruption_budget.yaml | Availability during updates |
| files/ | Configuration files (optional) |
| horizontal_pod_autoscaler.yaml | HPA (optional) |
| network_policy.yaml | Network rules (optional) |
