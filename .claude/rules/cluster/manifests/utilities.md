---
paths:
  - "cluster/manifests/utilities/**/*.yaml"
---

* Development environment utilities (redis, minio, memcached, etc.) referenced by other manifests
* Flat structure (no base/overlays)
* Referenced from other manifests as `overlays/dev/redis/`, `overlays/dev/minio/`, etc.
* When modifying utilities, find and update all consumers: `grep -r "utilities/{name}" cluster/manifests/**/kustomization.yaml`

## Labels

Utility base manifests use only `app.kubernetes.io/component` in selectors.
Do NOT add `app.kubernetes.io/name` — it is injected by consumer overlays via `includeSelectors: true`.

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

Knative Service utilities MUST NOT use `includeSelectors: true`.
Kustomize adds `spec.selector` to Knative Services (confusing `serving.knative.dev/v1 Service` with `v1 Service`), which is an invalid field and breaks the resource.

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

When using `namePrefix` with a Gateway and VirtualService, the VirtualService `gateways` reference and `destination.host` must use the full prefixed name (e.g., `memory-bank-cloudevents-ingress`).
Kustomize `namePrefix` does not auto-update cross-resource references in VirtualService specs.

### Required Patches

Consumers MUST create `patches/` directory.
Copy from existing consumer of the same utility type.

| Patch | When Required | Content |
|-------|---------------|---------|
| `patches/service.yaml` | Always | Deployment/StatefulSet: `trafficDistribution: PreferClose`; Knative: env overrides |
| `patches/pod_disruption_budget.yaml` | Deployment/StatefulSet only | `maxUnavailable: 1` |
| `patches/deployment.yaml` or `patches/stateful_set.yaml` | Deployment/StatefulSet only | Istio sidecar, topologySpread, resources, env vars |

Workload patches vary by utility (Istio annotations, zone spreading, sidecars).
Find existing consumer of the same utility and copy its patches.

### Redis Replica Count

`spec.replicas`, `metadata.annotations.REPLICAS`, and `metadata.annotations.QUORUM` in a consumer's `redis/patches/stateful_set.yaml` are one unit — change all three together, setting `QUORUM` to the majority of the replica count.

| Value | Wired by | Failure mode when wrong |
|-------|----------|-------------------------|
| `QUORUM` | `replacements` from `metadata.annotations.QUORUM` | No build error; liveness probe and haproxy health checks fail at runtime |
| `REDIS_REPLICAS` | `replacements` from `spec.replicas` | No build error; haproxy backend server list is wrong |
| `REPLICAS` | Not wired | None — kept equal to `spec.replicas` by convention only |

Read `cluster/manifests/utilities/redis/files/` for what consumes these rather than assuming.

At 1 replica sentinel failover is inoperative and the consumer holds a single copy.
Reduce only after checking what the consumer stores — read its source rather than assuming the `allkeys-lru` policy means everything is disposable, since consumers do persist TTL-less keys.

Consumers dialing `{parent}-redis-haproxy-reader` are safe at any count: `init-haproxy.sh` gates the `redis_slave` backend on `role:slave` only when more than one replica exists, so the lone master serves reads.

### Memcached Replica Count

`spec.replicas` and `metadata.annotations.REPLICAS` in a consumer's `memcached/patches/stateful_set.yaml` are one unit — change both together.

| Value | Wired by | Failure mode when wrong |
|-------|----------|-------------------------|
| `MEMCACHED_REPLICAS` | `replacements` from `spec.replicas` | No build error; mcrouter's pool omits a live server or lists one that never resolves, and its initContainer blocks on `getent hosts` |
| `REPLICAS` | Not wired | None — kept equal to `spec.replicas` by convention only |

`init-mcrouter.sh` defaults `OPERATION_POLICY` to `sync`, which routes every write to all servers in the pool and reads from one, so replicas hold identical copies instead of sharding.
Read `cluster/manifests/utilities/memcached/files/init-mcrouter.sh` for which policies shard rather than assuming the pool does.

Under `sync` and `async` the pool holds one replica's `-m`, which the container derives from its own `requests.memory`, so raising the count buys read failover and no capacity.
It also costs write availability, since `AllSyncRoute` answers with the worst child reply and every write then depends on all replicas being up, and it widens `liveness-probe.sh`'s `num_servers_down` check, which restarts mcrouter while any one of them is down.
Cutting the count is consequently the lever that frees memory without moving the cache ceiling — lowering `requests.memory` moves the ceiling itself, so read `.claude/reference/cluster/manifests/deriving-resource-values.md` before touching it.
At 1 replica a restart empties the cache and the consumer's own clients miss until it refills.

### NetworkPolicy Ingress from Consumer Pods

The consumer's `default-deny` blocks pod-to-utility traffic within its own namespace, so add an ingress NetworkPolicy for every utility its pods connect to.
A missing rule surfaces as an application-level connection failure, not a manifest error.

| Field | Value |
|-------|-------|
| `podSelector` | `app.kubernetes.io/name: {parent-app}` + `app.kubernetes.io/component: {utility-component}` |
| `from` | The consuming pods' selectors, in the consumer's own namespace |
| `ports` | The utility's containerPort, not the Service `port` |

Utility Services rename ports.
Read the containerPort behind the Service's named `targetPort`, then confirm it against the port the process binds — a declared containerPort can be wrong (`redis` declares 6379 for `sentinel`, which listens on 26379).

Scope both the ports and the components to what the consumer's configuration actually dials.
A utility shipping several Services or components (`redis` has `-writer`/`-reader` Services and `redis`/`redis-haproxy` pods) does not justify covering all of them, and traffic between its own components is already allowed by its own `network_policy.yaml`.
A surplus rule is not harmless — consumers may run `ALLOW_ANY`, where the NetworkPolicy is the only enforcement.

When the utility's Service carries `istio.io/use-waypoint`, also allow HBONE 15008 — see `Istio Ambient Mesh (Waypoint Proxy)` in `cluster/manifests.md`.

Example: `cluster/manifests/whisper-worker/overlays/dev/network_policy.yaml`

### NetworkPolicy Egress for Istio Sidecars

When a utility has `policyTypes: Egress` in its NetworkPolicy and Istio sidecar is enabled, cross-namespace egress rules (istio-system/istiod, otel/otel-agent) must be defined in the consumer's overlay, not in the utility.

Kustomize `labels includeSelectors: true` adds consumer labels to all `podSelector` fields including egress targets.
This breaks cross-namespace egress because target pods do not have consumer-specific labels.

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
