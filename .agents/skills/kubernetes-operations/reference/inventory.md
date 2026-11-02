# Cluster-Wide Inventory

Enumerating workloads, containers and their resources with `kubectl`, and resolving a Pod back to what owns it.

## Reading the Patched Spec Across Overlays

Piping many overlays' `kustomize build` output through `kubectl apply --dry-run=client -o json -f -` to read back the effective (patched) spec returns the bare resource when the stream holds exactly one document but wraps it in a `kind: List` with `.items[]` once it holds more than one, and aborts the whole batch with empty stdout and `applying patch locally: expected a struct, but received a nil` if any document in the stream is a CustomResourceDefinition — filtering the build output to `kind: Deployment|StatefulSet|DaemonSet` before piping it in avoids both, confirmed across 167 overlay roots here where 20 failed on the CRD error until filtered.

## Restart Count Alone Is Not Incident Evidence

A `RESTARTS` count in `kubectl get pods` accumulates over the container's whole lifetime, so a high count does not by itself mean a problem is still live.

| Cause | Signal | How to confirm |
|-------|--------|-----------------|
| Node/cluster reboot | Static Pods and every Pod waiting on a dependency that was also restarting all restart within the same narrow window | `lastState.terminated.startedAt`/`finishedAt` cluster together across unrelated Pods |
| Liveness probe restarts itself on a schedule | Restart count climbs steadily with no other symptom | Read the mounted ConfigMap for the actual probe command — confirmed here on `fluentd-aggregator`, whose probe script exits by design once a day |
| HorizontalPodAutoscaler scale-down | A terminating Pod's own readiness probe fails right after a `Killing` event | The Deployment/ReplicaSet's own events show `SuccessfulRescale`/`SuccessfulDelete` immediately around the probe failure |
| Controller reconcile conflict on an unrelated object | A namespace Warning event names a different object, e.g. a Pod annotation update | The named Pod's own `containerStatuses[].restartCount` is unaffected |
| Knative Revision rollout | Readiness probe / `FailedPreStopHook` Warning events on `*-<N>-deployment-*` pods, paired with an older or newer Revision generation replacing it | The named Pod's own generation no longer exists (or the other generation does), and the current stable generation's Pods are `Running`/`Ready` with 0 restarts — the new generation's Istio sidecar not yet listening on port 15020/15021 explains the readiness failure, confirmed independently on 2026-09-13, 2026-09-27 and 2026-09-30 here |
| Cilium kvstore quorum failure (`cilium-etcd`/control-plane `etcd` slow disk I/O) | Restarts spike simultaneously across unrelated leader-election-based controllers (cert-manager, tidb-operator, knative-* controllers/webhooks, github-actions-*-controller, grafana-manifest-controller, vpa-recommender/updater, etc.), and some Pods are stuck `Pending` with `FailedCreatePodSandBox: unable to connect to Cilium agent` | `kubectl logs -n kube-system <cilium-agent-pod>` shows `Kvstore service is not ready: ... quorum check failed`; the node carries a `node.cilium.io/agent-not-ready` taint (`kubectl get node <node> -o jsonpath='{.spec.taints}'`); both `cilium-etcd-0-0` and the control-plane `etcd-minikube` answer simple Puts/applies in seconds instead of milliseconds with no `limits.cpu` on either container, so the cause is host disk I/O rather than CPU — confirmed independently on 2026-08-27 (GitHub issue #5206) and 2026-10-03/04 here |

Rule each of these out against the object's own recent events and `lastState.terminated` timestamps before treating a restart count as an active incident.

## Reclaiming Over-Provisioned Memory Requests

Enumerate `resources.requests.memory` from the workload objects first, then again from live Pods.
An admission-injected container (`istio-proxy`/`istio-validation`) carries its own `requests.memory`, invisible on the owning Deployment/StatefulSet/DaemonSet for the same reason its other `resources` fields are (see `## Non-Interactive Access` in `.claude/skills/kubernetes-operations/reference/queries.md`), so a workload-object-only enumeration misses it.

```bash
# Workload objects (Deployment/StatefulSet/DaemonSet, all namespaces) — catches a 0-replica workload, misses an injected sidecar
kubectl get deployments,statefulsets,daemonsets -A -o json | jq -r '
  .items[] |
  .kind as $kind | .metadata.namespace as $ns | .metadata.name as $name |
  (.spec.template.spec.containers[]? // empty) |
  select(.resources.requests.memory != null) |
  [$ns, $kind, $name, .name, .resources.requests.memory] | @tsv
'

# Live Pods — catches an injected sidecar, misses a 0-replica workload
kubectl get pods -A -o json | jq -r '
  .items[] |
  .metadata.namespace as $ns | .metadata.name as $pod |
  ((.spec.containers[]? // empty), (.spec.initContainers[]? // empty)) |
  select(.resources.requests.memory != null) |
  [$ns, $pod, .name, .resources.requests.memory] | @tsv
'
```

Run both and union the results for exhaustive cluster-wide coverage — confirmed here, where the Pod pass surfaced 16 `istio-proxy`/`istio-validation` containers carrying `requests.memory` (ranging 64Mi–256Mi, set per-workload via the `sidecar.istio.io/proxyMemory`/`proxy.istio.io/config` annotation rather than one cluster-wide default) that were absent from the workload-object pass.

Exclude a VPA `updateMode: Auto` target before treating a container as a manual-edit candidate — it already rewrites its own `requests.memory` from observed usage on its own schedule, so a manually-lowered value is overwritten rather than reclaimed (`## resizePolicy` in `.claude/rules/cluster/manifests.md` covers the manifest-side consequence; this is the live-cluster enumeration):

```bash
kubectl get vpa -A -o json | jq -r '.items[] | [.metadata.namespace, .spec.targetRef.kind, .spec.targetRef.name, .spec.updatePolicy.updateMode] | @tsv'
```

Join on `(namespace, targetRef.kind, targetRef.name)`, not on the container name — confirmed here, where the container each of several `*-minio` VPA targets manages (`assets-minio`, `cortex-bot-minio`, `drive-sync-minio`, and others) is named `minio` identically in every one of them.

Checking whether a container's restart was actually memory-driven, not merely killed, needs the container status rather than the Pod's own phase:

```bash
kubectl get pod <pod> -n <namespace> -o jsonpath='{range .status.containerStatuses[*]}{.name}{"\t"}{.lastState.terminated.reason}{"\t"}{.lastState.terminated.exitCode}{"\n"}{end}'
```

`exitCode: 137` alone is not evidence of an OOM kill — confirmed here on a live `istio-proxy` sidecar whose `lastState.terminated` carried `exitCode: 137` but `reason: "Error"`, not `reason: "OOMKilled"`, from an unrelated SIGKILL.
`reason` has to read exactly `OOMKilled` to attribute a restart to memory pressure.

Once a candidate is confirmed idle in Mimir (`container_memory_working_set_bytes`, see `## PromQL (Mimir)` in `.claude/skills/kubernetes-operations/reference/queries.md`) and not VPA-managed, deriving the new value — and confirming the manifest is even allowed to carry `resources` at all — is a separate step: see `.claude/reference/cluster/manifests/deriving-resource-values.md` and `## When to Define Resources` in `.claude/rules/cluster/manifests.md`.

Locating which `cluster/manifests/{app}/` directory actually sets a candidate's value needs every `overlays/dev/kustomization.yaml`'s own `namespace:` field, not just the directories carrying their own `namespace.yaml` — several apps share a namespace with a sibling app that has no `namespace.yaml` of its own, confirmed here where the `mimir` namespace is served by both `cluster/manifests/mimir/` and `cluster/manifests/prometheus-adapter/`, and `otel` by both `cluster/manifests/otel-agent/` and `cluster/manifests/otel-collector/`; grepping only the directory matching the namespace's own `namespace.yaml` misses the sibling's containers entirely.

## Tracing a Pod to Its Owner

Not every Pod resolves through Deployment/StatefulSet/DaemonSet.

| `metadata.ownerReferences[0].kind` | Resolves to | Notes |
|---|---|---|
| `ReplicaSet` | One more hop: the ReplicaSet's own `ownerReferences[0]` names the Deployment | a Knative Revision's generated Deployment satisfies this same chain — check the Deployment's own `ownerReferences` for `kind: Revision` to exclude it (see `## Non-Interactive Access` in `.claude/skills/kubernetes-operations/reference/queries.md`) |
| `StatefulSet` | The StatefulSet itself, no ReplicaSet hop | tidb-operator's `pd`/`tikv`/`tidb`/`ticdc` component pods land here too — the operator creates an ordinary StatefulSet rather than owning pods from the TidbCluster CR directly |
| `DaemonSet` | The DaemonSet itself, no ReplicaSet hop | |
| `StrimziPodSet` | The StrimziPodSet itself | confirmed here (`knative-eventing`, `mimir`, `slack-logger` Kafka pools) — these Kafka pods never pass through a standard controller |
| `VitessShard` / `EtcdLockserver` | The CR itself | confirmed here (`mattermost`) — same as StrimziPodSet, no standard controller in the chain |
| `Node` | A static (mirror) pod, not scheduled through the API server | confirmed on `kube-controller-manager-minikube`/`kube-scheduler-minikube` in `kube-system`; also carries a `kubernetes.io/config.mirror` annotation no other owner kind does |
| `Job` | The Job itself | the Job's own `ownerReferences` is empty for a standalone/hook Job, names a `CronJob` for a scheduled one, or names another controller that created the Job directly — confirmed here: the minio bucket-seeding `*-mc` Jobs carry no owner at all, `grafana-cache-warmer`/`descheduler`/`promphet` are `CronJob`-owned, and a `github-actions-runner-controller` cache-cleanup Job and `slack-logger`'s `tidb-initializer` Job are owned by `ScaleSet`/`TidbInitializer` respectively |
