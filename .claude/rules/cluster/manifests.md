---
paths:
  - "cluster/manifests/**/*.yaml"
---

* Production manifests in `manifests/`, development in `skaffold/`
* Use singular resource type with underscores (`service_account.yaml`)
* Pin images by digest in kustomization.yaml, not by tag
* When using external images, add mirroring job to `.github/workflows/99_mirroring.yaml` (do not reference external images directly)
* In kustomization.yaml `images` section, set `name` to match the image reference in manifests (what Kustomize searches for), `newName` to the actual registry path (what Kustomize replaces with)

## Image References

| Image Type | `name` | `newName` |
|------------|--------|-----------|
| Internal (this repo) | `ghcr.io/hippocampus-dev/hippocampus/{app}` | `ghcr.io/hippocampus-dev/hippocampus/{app}` |
| External (mirrored) | Original image name (e.g., `nginx`, `redis/redis-stack`) | `ghcr.io/hippocampus-dev/hippocampus/mirror/{original}` |

When adding, moving, or removing an internal image reference, update the corresponding GitHub Actions workflow:

1. Find the workflow: `grep -l "{image-name}" .github/workflows/00_*.yaml`
2. Update `env.KUSTOMIZATION` with repository-root-relative path(s) (comma-separated, no spaces)

## Secret Management

When a Deployment references a Secret (via `envFrom.secretRef` or `env.valueFrom.secretKeyRef`), ensure the Secret is properly configured:

| Secret Source | Required Configuration |
|---------------|------------------------|
| Vault | Create `secrets_from_vault.yaml` in `overlays/dev/` |
| ConfigMapGenerator | Use `secretGenerator` in `kustomization.yaml` |

For Vault secrets, add `secrets_from_vault.yaml` to `kustomization.yaml` resources:

```yaml
# secrets_from_vault.yaml
apiVersion: kustomize.kaidotio.github.io/v1
kind: SecretsFromVault
metadata:
  name: {app-name}
spec:
  vaultSecrets:
    - path: /kv/data/{app-name}
      key: {SECRET_KEY}
```

Example: `cluster/manifests/cortex-api/overlays/dev/secrets_from_vault.yaml`

## Container Defaults

* Consistency with existing patterns takes precedence over minimizing fields
* When adding fields, check similar files in the repository and match their style
* When defining `resources`, always specify both `requests` and `limits` (Kubernetes copies limits to requests when requests are omitted, causing scheduling issues)
* All containers require secure defaults (non-root UID 65532, no privilege escalation, read-only filesystem, emptyDir with `medium: Memory` for `/tmp` if application needs writable temp)
* Set `dnsConfig.options` with `ndots: "1"` to prevent unnecessary DNS search domain expansion (all services use FQDNs, not short names like `service.namespace`). The path varies by workload type:

| Workload | dnsConfig Path |
|----------|----------------|
| Deployment, StatefulSet, DaemonSet | `spec.template.spec.dnsConfig` |
| Job | `spec.template.spec.dnsConfig` |
| CronJob | `spec.jobTemplate.spec.template.spec.dnsConfig` |
| Istio Gateway (IstioOperator-managed) | Via `overlays` patches in `cluster/bin/deploy-istio.sh`, not Kustomize |

Exception: node-local-dns uses `dnsPolicy: Default` with `hostNetwork: true` for node-level DNS caching
* Use `restricted` Pod Security Standard by default; use `baseline` or `privileged` only with documented reason in namespace.yaml comment
* All workloads require standard labels (`app.kubernetes.io/name`, `app.kubernetes.io/component`)
* When avoiding hardcoded values: use Kustomize (replacements, configMapGenerator) for cross-resource references, Downward API for own pod metadata (name, labels, resources), scripts/API calls as last resort (example: `cluster/manifests/utilities/redis/` and `cluster/manifests/translator/overlays/dev/redis/kustomization.yaml`)

## Pod Metadata (Labels and Annotations)

Pod metadata is defined in `pod.metadata` within deployment/statefulset/daemonset specs.

### Istio Sidecar Injection

When a workload needs Istio sidecar injection, add the label and proxy resource annotations to `pod.metadata.labels` and `pod.metadata.annotations`:

```yaml
spec:
  template:
    metadata:
      labels:
        sidecar.istio.io/inject: "true"
      annotations:
        sidecar.istio.io/proxyCPULimit: 1000m
        sidecar.istio.io/proxyMemoryLimit: 1Gi
        sidecar.istio.io/proxyCPU: 30m
        sidecar.istio.io/proxyMemory: 64Mi
```

| Annotation | Value | Purpose |
|------------|-------|---------|
| `sidecar.istio.io/inject` | `"true"` | Enable Istio sidecar injection |
| `sidecar.istio.io/proxyCPULimit` | `1000m` | CPU limit for the proxy container |
| `sidecar.istio.io/proxyMemoryLimit` | `1Gi` | Memory limit for the proxy container |
| `sidecar.istio.io/proxyCPU` | `30m` | CPU request for the proxy container |
| `sidecar.istio.io/proxyMemory` | `64Mi` | Memory request for the proxy container |

Omit these annotations if the namespace has `istio-injection=disabled` label or if the workload must run without a sidecar (e.g., `hostNetwork: true` workloads are incompatible with Istio sidecar).

Examples: `cluster/manifests/bakery/overlays/dev/patches/deployment.yaml`

### Istio Resource workloadSelector

When creating Istio resources (PeerAuthentication, Sidecar, Telemetry) for multi-component services (e.g., DaemonSet + StatefulSet, multiple Deployments), use only `app.kubernetes.io/name` in `workloadSelector` to apply the policy to all components with a single resource:

| Service Structure | workloadSelector Labels |
|-------------------|------------------------|
| Single component | `app.kubernetes.io/name` (optionally with `app.kubernetes.io/component`) |
| Multiple components sharing same policy | `app.kubernetes.io/name` only |
| Multiple components with different policies | Separate resources per component with both `name` and `component` |

Examples: `cluster/manifests/kube-state-metrics/overlays/dev/sidecar.yaml` (applies to both DaemonSet and StatefulSet)

## Container Field Order

Container fields must follow this order (omit fields not needed):

```
name → securityContext → image → imagePullPolicy → command → args → env → resources → ports → startupProbe → readinessProbe → livenessProbe → lifecycle → volumeMounts
```

Probe fields follow this order:

```
exec/httpGet/tcpSocket → initialDelaySeconds → periodSeconds → successThreshold → failureThreshold → timeoutSeconds
```

## Runtime Thread Count and cgroup

Rust (tokio) and Go runtimes determine worker thread count based on detected CPU cores. Without proper configuration, the runtime may spawn threads for all node CPUs, causing excessive context switching when the container is throttled.

**Only applies to Go and Rust applications.** Check `cluster/applications/{app}/Dockerfile` to identify the runtime:
- Go: `FROM golang:*`
- Rust: `FROM rust:*` or `FROM ghcr.io/rust-lang/rust:*`
- Not applicable: `nginx`, `python`, `node`, external images

| Language | Required Configuration |
|----------|------------------------|
| Go | `resources.limits.cpu` + `GOMAXPROCS` env via `resourceFieldRef: limits.cpu` |
| Rust (tokio) | `resources.requests.cpu` (cgroup auto-detection) |

### Go

In `base/deployment.yaml`, define ONLY the GOMAXPROCS/GOMEMLIMIT env vars (no resources section):

```yaml
env:
  - name: GOMAXPROCS
    valueFrom:
      resourceFieldRef:
        resource: limits.cpu
  - name: GOMEMLIMIT
    valueFrom:
      resourceFieldRef:
        resource: limits.memory
```

In `overlays/dev/patches/deployment.yaml`, define ALL resources:

```yaml
containers:
  - name: {container-name}
    resources:
      requests:
        cpu: 10m
        memory: 64Mi
      limits:
        cpu: 1000m
        memory: 128Mi
```

The `resourceFieldRef` works at runtime (Kubernetes reads limits from the final merged manifest), not at Kustomize build time. Patches merge into base before deployment.

### Rust (tokio)

Set `resources.requests.cpu` to enable cgroup-based detection via `available_parallelism()`:

```yaml
resources:
  requests:
    cpu: 5m
```

## OpenTelemetry Environment Variables

When adding OpenTelemetry tracing to a container, include all required environment variables. Do NOT add only a subset.

### Common (all languages)

| Variable | Value Pattern |
|----------|---------------|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `http://otel-agent.otel.svc.cluster.local:4317` |
| `OTEL_SERVICE_NAME` | Downward API: `metadata.labels['app.kubernetes.io/name']` |
| `OTEL_TRACES_SAMPLER` | `parentbased_traceidratio` |
| `OTEL_TRACES_SAMPLER_ARG` | `"1.0"` |
| `OTEL_BSP_SCHEDULE_DELAY` | `"5000"` |
| `OTEL_BSP_EXPORT_TIMEOUT` | `"30000"` |
| `OTEL_BSP_MAX_QUEUE_SIZE` | `"2048"` |
| `OTEL_BSP_MAX_EXPORT_BATCH_SIZE` | `"512"` |

### Go only (add after common variables)

| Variable | Value Pattern |
|----------|---------------|
| `OTEL_GO_X_EXEMPLAR` | `"true"` |
| `OTEL_METRICS_EXEMPLAR_FILTER` | `trace_based` |

Example: `cluster/manifests/reporting-server/overlays/dev/patches/deployment.yaml` (Go), `cluster/manifests/embedding-gateway/overlays/dev/patches/deployment.yaml` (Python)

## Container Ports

* Always specify `name` for containerPort (Service's `targetPort` references by name, not number)
* Always specify `protocol: TCP` explicitly (even though it's the default)
* Define `metrics` port only if the application exposes its own metrics endpoint (Istio sidecar metrics via port 15020 don't require containerPort definition)

| Port Name | Purpose |
|-----------|---------|
| `http` | HTTP service |
| `https` | HTTPS service |
| `grpc` | gRPC service |
| `metrics` | Prometheus metrics (app-specific) |

## Topology Spread Constraints

When using `matchLabelKeys` in `topologySpreadConstraints`, use the correct label for each workload type:

| Workload | matchLabelKeys | Reason |
|----------|----------------|--------|
| Deployment | `pod-template-hash` | Auto-generated by ReplicaSet controller |
| StatefulSet | `controller-revision-hash` | Auto-generated by StatefulSet controller |

Using the wrong label causes `matchLabelKeys` to be ignored (the label does not exist on pods).

## HPA-Managed Workloads

When a workload has a HorizontalPodAutoscaler (HPA), do NOT include `replicas` field in Kustomize patches. ArgoCD selfHeal reverts HPA scaling changes when the patch defines an explicit replica count.

| HPA Configured | Kustomize Patch |
|----------------|-----------------|
| Yes | Do NOT include `replicas` field |
| No | Include `replicas` field |

Example: `cluster/manifests/fluentd/overlays/dev/patches/stateful_set.yaml` (HPA manages replicas, patch omits `replicas`)

## resizePolicy

Only add `resizePolicy` when VPA (VerticalPodAutoscaler) with `updateMode: Auto` is configured for the workload.

| VPA Configuration | resizePolicy |
|-------------------|--------------|
| `updateMode: Auto` | Required |
| `updateMode: Off` / No VPA | Do not add |

Example: `cluster/manifests/utilities/minio/` (has both `vertical_pod_autoscaler.yaml` and `resizePolicy` in `stateful_set.yaml`)

## Pod Security Standards

| Level | When to Use | Comment Required |
|-------|-------------|------------------|
| `restricted` | Default for all namespaces | No |
| `baseline` | NFS volumes, GPU workloads, Istio Gateway/Waypoint, specific runtime requirements | Yes |
| `privileged` | eBPF, host network, Kaniko/Docker-in-Docker | Yes |

When using `baseline` or `privileged`, add a comment in `namespace.yaml` explaining the reason:

```yaml
labels:
  # whisper-worker uses nfs
  pod-security.kubernetes.io/enforce: baseline
```

### Capabilities

Do not add Linux capabilities that are unnecessary in Kubernetes:

| Capability | Kubernetes Context | Action |
|------------|-------------------|--------|
| `IPC_LOCK` | Swap is disabled by default | Do not add |
| `NET_ADMIN` | Required only for network namespace manipulation | Add only if needed |
| `SYS_PTRACE` | Required only for debugging/profiling | Add only if needed |

For `restricted` compliance, always drop all capabilities: `capabilities: { drop: [ALL] }`

## Graceful Shutdown (preStop)

Applications with `lifecycle.preStop` sleep allow Kubernetes Endpoints to propagate before SIGTERM processing. However, if the application implements lameduck internally, preStop sleep is redundant.

| Application Type | preStop Required |
|------------------|------------------|
| External images (nginx, redis, etc.) | Yes |
| controller-runtime (webhooks, controllers) | Yes |
| Go/Rust/Python with internal lameduck | No |

### Checking for Lameduck Implementation

Search for lameduck pattern in source code:
- Go: `time.Sleep(lameduck)` after `signal.Notify(quit, syscall.SIGTERM)`
- Rust: `time::sleep(lameduck)` after signal handling

### When Lameduck is Implemented

Comment out preStop and add explanation:

```yaml
          # {app-name} implements lameduck
          #lifecycle:
          #  preStop:
          #    exec:
          #      command: ["sleep", "3"]
```

Example: `cluster/manifests/bakery/base/deployment.yaml`, `cluster/applications/anonymous-proxy/manifests/deployment.yaml`

## Prometheus Metrics Scraping

When Istio sidecar is enabled (`sidecar.istio.io/inject: "true"`), Prometheus scrapes metrics via port 15020 (Istio merged metrics), NOT the application port.

| Istio Sidecar | Prometheus Scrape Port | NetworkPolicy |
|---------------|------------------------|---------------|
| Enabled | 15020 | `allow-envoy-stats-scrape` only |
| Disabled | Application port (e.g., 8080) | App-specific NetworkPolicy |

Do NOT add separate NetworkPolicy for application metrics port when using Istio sidecar. The `allow-envoy-stats-scrape` NetworkPolicy (port 15020) handles all Prometheus scraping.

```yaml
# Correct: allow-envoy-stats-scrape covers all pods with sidecar
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-envoy-stats-scrape
spec:
  podSelector: {}
  ingress:
    - from:
        - namespaceSelector:
            matchLabels:
              name: prometheus
          podSelector:
            matchLabels:
              app.kubernetes.io/name: prometheus
              app.kubernetes.io/component: ""
      ports:
        - protocol: TCP
          port: 15020
```

## Service-to-Service Communication

When adding or removing service-to-service communication, update both sides:

| Side | Configuration |
|------|---------------|
| Caller | Sidecar egress (+ ServiceEntry for external hosts) |
| Callee | NetworkPolicy ingress (+ AuthorizationPolicy if using Istio) |

### AuthorizationPolicy Path Matching

Istio AuthorizationPolicy `paths` field matches against the path only, not the full URI. Query strings are stripped before evaluation.

| URI Design | AuthorizationPolicy Compatibility |
|------------|-----------------------------------|
| Path parameters (`/resource/{id}`) | Compatible - paths can use prefix or exact match |
| Query parameters (`/resource?id=...`) | Not compatible - query string is stripped, all requests to `/resource` match |

When an API endpoint needs path-based authorization, use path parameters instead of query parameters.

### Istio Ambient Mesh (Waypoint Proxy)

Use Ambient Mesh when sidecar proxy is incompatible with the workload (e.g., Kaniko cannot run with UID 1337 required by istio-proxy).

When a caller uses Ambient Mesh (`istio.io/dataplane-mode: ambient`), the destination service needs a waypoint proxy.

**Callee configuration:**
1. Add `waypoint.yaml` (Gateway with `gatewayClassName: istio-waypoint`)
2. Add `istio.io/use-waypoint: waypoint` label to Service
3. Allow HBONE port 15008 in NetworkPolicy

When the destination service has `waypoint.yaml`, the NetworkPolicy must allow HBONE port 15008 in addition to the application port:

```yaml
# waypoint.yaml
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: waypoint
spec:
  gatewayClassName: istio-waypoint
  listeners:
    - name: mesh
      port: 15008
      protocol: HBONE
```

```yaml
# network_policy.yaml
ingress:
  - from:
      - namespaceSelector:
          matchLabels:
            name: caller-namespace
    ports:
      - protocol: TCP
        port: 8080
      # HBONE
      - protocol: TCP
        port: 15008
```

| Destination has `waypoint.yaml` | NetworkPolicy ports |
|---------------------------------|---------------------|
| Yes | Application port + HBONE (15008) |
| No | Application port only |

Examples: `cluster/manifests/cortex-api/overlays/dev/`, `cluster/manifests/token-request-server/overlays/dev/`, `cluster/manifests/httpbin/overlays/dev/`

### Sidecar Mode Selection

| Condition | Mode | Reason |
|-----------|------|--------|
| All destinations are known cluster services | `REGISTRY_ONLY` | Least privilege, explicit egress |
| Connects to node IPs (kubelets, host endpoints) | `ALLOW_ANY` | Node IPs are not in service registry |
| Connects to dynamic/arbitrary external hosts | `ALLOW_ANY` | Cannot enumerate all destinations |
| Manages arbitrary cluster resources (operators, dispatchers) | `ALLOW_ANY` | Destinations change dynamically |

Default to `REGISTRY_ONLY`. Use `ALLOW_ANY` only when destinations cannot be enumerated in egress hosts.

### Sidecar Egress (REGISTRY_ONLY mode)

Use specific FQDNs for least privilege:

| Destination | Format | Example |
|-------------|--------|---------|
| Same namespace | `./{service}.{namespace}.svc.cluster.local` | `./mimir-etcd.mimir.svc.cluster.local` |
| Cross namespace | `{namespace}/{service}.{namespace}.svc.cluster.local` | `otel/otel-agent.otel.svc.cluster.local` |
| Knative Service | `{namespace}/*` (wildcard required for per-Revision services) | `alerthandler/*` |
| External host | `./external-host.com` + ServiceEntry | `./api.github.com` |

Use `./` prefix for services in the same namespace (peer communication, utility services). Use `{namespace}/` for cross-namespace access.

Avoid `{namespace}/*` for non-Knative services - it allows access to all services in that namespace.

### Resource Naming

| Pattern | Resources | Example |
|---------|-----------|---------|
| App/component name only | PeerAuthentication, Sidecar, Telemetry, AuthorizationPolicy, Gateway, VirtualService | `varnish`, `cortex-api` |
| `{app}-{feature}` | EnvoyFilter, VirtualService (specific routes) | `varnish-access-log`, `embedding-gateway-purge` |
| `{parent-app}-{utility}` | NetworkPolicy, VirtualService (utility routing) | `cortex-api-redis`, `assets-minio` |
| FQDN | ServiceEntry, VirtualService (cross-namespace) | `api.github.com`, `*.svc.cluster.local` |

Utility names match directory names under `cluster/manifests/utilities/`.

Do not use descriptive suffixes like `-allow-purge` or `-deny-external-cache-invalidation` for single-purpose resources.

### External Egress

When accessing external hosts, both Sidecar and ServiceEntry are required:

```yaml
apiVersion: networking.istio.io/v1
kind: ServiceEntry
metadata:
  name: api.github.com
spec:
  exportTo:
    - .
  hosts:
    - api.github.com
  location: MESH_EXTERNAL
  ports:
    - name: https
      number: 443
      protocol: HTTPS
  resolution: DNS
```

Examples: `cluster/manifests/runner/overlays/dev/service_entry.yaml`, `cluster/manifests/alerthandler/overlays/dev/service_entry.yaml`

## Proxy-WASM Filters

When deploying proxy-wasm filters, prefer WasmPlugin over EnvoyFilter:

| Use Case | Choice | Reason |
|----------|--------|--------|
| Default | WasmPlugin | Simpler, fewer lines of YAML |
| Need `INSERT_BEFORE` specific filter | EnvoyFilter | WasmPlugin only supports phase-based ordering |
| Need to insert before `istio.metadata_exchange` | EnvoyFilter | WasmPlugin cannot insert before this filter |

Example: `cluster/manifests/embedding-gateway/overlays/dev/wasm_plugin.yaml` (WasmPlugin), `cluster/manifests/httpbin/overlays/dev/envoy_filter.yaml` (EnvoyFilter)

## Sub-Component Placement

When a service depends on another component, decide whether it should be a sub-directory under the parent or a standalone ArgoCD Application:

| Relationship | Placement | Example |
|--------------|-----------|---------|
| Tightly coupled (exists only for the parent) | Sub-directory under parent's `overlays/dev/{component}/` | `memory-bank/overlays/dev/cloudevents-relay/` |
| Shared infrastructure (redis, minio, etc.) | Sub-directory referencing `cluster/manifests/utilities/` | `memory-bank/overlays/dev/qdrant/` |
| Independent service (own lifecycle) | Standalone `cluster/manifests/{service}/` with own ArgoCD Application | `bakery/` |

Sub-components share the parent's namespace and ArgoCD Application. Each sub-component directory contains its own `kustomization.yaml`, Istio resources (peer_authentication, sidecar, telemetry), and `patches/`.

| Sub-component Type | kustomization.yaml `resources` | `namePrefix` | `labels` |
|--------------------|--------------------------------|--------------|----------|
| Utility (from `cluster/manifests/utilities/`) | `../../../../utilities/{utility}` | `{parent}-` | `app.kubernetes.io/name: {parent}`, `variant: utilities` |
| Application-specific (from `cluster/applications/`) | `../../../../../applications/{component}/manifests` | Not required (uses application's own labels) | Application's own labels |

When adding a sub-component, also update the parent's ArgoCD Application `manifest-generate-paths` annotation to include the sub-component's source directory.

Examples: `cluster/manifests/memory-bank/overlays/dev/qdrant/` (utility), `cluster/manifests/memory-bank/overlays/dev/cloudevents-relay/` (application-specific)

## ArgoCD Application

After creating manifests for any workload, create an ArgoCD Application:
  Read: `.claude/rules/cluster/manifests/argocd-applications.md`

## Workload Types

| Workload | Description |
|----------|-------------|
| External Service | HTTP services exposed via Istio Gateway |
| Internal Service | Cluster-internal HTTP services |
| Stateful | StatefulSet for databases, caches |
| Daemon | DaemonSet for node-level agents |
| CronJob | Scheduled periodic tasks |
| Job | One-time tasks, ArgoCD hooks |
| Knative Service | Serverless HTTP (scale to zero) |
| Knative Eventing | Event-driven with Broker/Trigger |

## Reference

If deploying an open-source GitHub project:
  Read: `.claude/rules/.reference/cluster/manifests/deploying-opensource-projects.md`

If creating a workload, choose the appropriate pattern:
  Read: `.claude/rules/.reference/cluster/manifests/workloads/external-service.md`
  Read: `.claude/rules/.reference/cluster/manifests/workloads/internal-service.md`
  Read: `.claude/rules/.reference/cluster/manifests/workloads/stateful.md`
  Read: `.claude/rules/.reference/cluster/manifests/workloads/daemon.md`
  Read: `.claude/rules/.reference/cluster/manifests/workloads/cronjob.md`
  Read: `.claude/rules/.reference/cluster/manifests/workloads/job.md`
  Read: `.claude/rules/.reference/cluster/manifests/workloads/knative-service.md`
  Read: `.claude/rules/.reference/cluster/manifests/workloads/knative-eventing.md`
