# JVM Heap Sizing

How to bound the heap of a JVM workload, whether the image is upstream or operator-managed.

## Why a Limit Is Required

`UseContainerSupport` is on by default, so the JVM reads the cgroup memory limit itself — no Downward API wiring is needed.
But without `resources.limits.memory` there is no cgroup limit to read, and the JVM falls back to the node's total memory and takes `MaxRAMPercentage` (default 25%) of that as the heap ceiling.
On a large node the ceiling reaches many gigabytes, the JVM never feels GC pressure, and RSS grows far past what the workload needs.
An absolute `-Xmx` caps the heap regardless of any cgroup limit, which is why it is the alternative below rather than a second precondition.

## Approaches

Bound the heap one of three ways:

| Approach | When |
|----------|------|
| `resources.limits.memory` + `-XX:MaxRAMPercentage` | Upstream images that do not pin the heap themselves — the heap tracks the limit, so the two stay in sync |
| Absolute `-Xms`/`-Xmx` (via the image's own options env var) | The chart or image already exposes a heap setting; then `limits.memory` is optional |
| Leave to the operator | Strimzi sizes the heap from the CR's own `resources`, by two different mechanisms: brokers get an injected `-Xms`/`-Xmx` set equal to each other — committing the full heap at startup, so RSS holds the whole `-Xmx` even when the live set is far smaller (force GC and re-read to find the live set) — which then wins over any `MaxRAMPercentage` added in a patch, while its operator containers ship baked-in percentage flags that read the cgroup limit. Either way set `resources` in the CR, not in a container patch — read the running process rather than assuming which mechanism applies. To avoid the up-front commit on a broker, set `spec.kafka.jvmOptions` with `-Xms` below `-Xmx` so the heap grows lazily to the live set; supplying only `-Xms` (no `-Xmx`) makes Strimzi inject no `-Xmx` at all, dropping the ceiling to the JVM's default 25% `MaxRAMPercentage` of each pool's own limit — tighter than Strimzi's 50%, and per-pool when broker and controller pools differ |

For the percentage approach:

```yaml
containers:
  - name: {container-name}
    env:
      - name: JAVA_TOOL_OPTIONS
        value: -XX:MaxRAMPercentage=40 ...
    resources:
      requests:
        cpu: 50m
        memory: 512Mi
      limits:
        memory: 768Mi
```

Derive both numbers rather than copying them: set `limits.memory` above the observed peak `container_memory_working_set_bytes` with headroom, and read sibling workloads in the same overlay for the prevailing spread.
When the observed peak already sits against the current limit, the circularity clause in `.claude/reference/cluster/manifests/deriving-resource-values.md` governs instead, and it withholds the raise altogether rather than merely redirecting where the number comes from.
Lower the percentage when the workload allocates direct buffers heavily.

## Non-Heap Memory

The remainder covers metaspace, code cache, thread stacks and GC structures.
Direct byte buffers do NOT come out of it: with `MaxDirectMemorySize` unset the JVM derives that ceiling from the max heap, so the percentage effectively reserves the heap twice.
For a workload that allocates direct buffers heavily (Kafka and Netty clients do), the two ceilings can therefore sum past the limit while each looks safe alone.
Pin `-XX:MaxDirectMemorySize` only against a measured figure (`jvm_buffer_pool_used_bytes`, or `-XX:NativeMemoryTracking=summary` read via `jcmd VM.native_memory summary`) — a guessed cap trades headroom for an `OutOfMemoryError: Direct buffer memory`, and an unreached ceiling is not itself consumption.

Unlike Go's `GOMEMLIMIT`, do NOT source the heap size from `resourceFieldRef`.
Its `divisor` accepts only unit conversions (`1`, `1Ki`, `1Mi`, `1Gi`, …), never an arbitrary multiple, so the value it yields is the limit itself in different units — the heap would equal the hard limit and leave nothing for non-heap.

## Example

Copy from: `cluster/manifests/knative-eventing-kafka/overlays/dev/patches/deployment.yaml` (percentage), `cluster/manifests/adhoc/elasticsearch/patches/deployment.yaml` (absolute `-Xms`/`-Xmx` via `ES_JAVA_OPTS`)
