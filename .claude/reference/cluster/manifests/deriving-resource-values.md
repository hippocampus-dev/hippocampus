# Deriving Resource Values from Observed Usage

How to turn an observation of a running workload into a `requests`/`limits` value without oscillating or reasoning in a circle.

## Choosing the Observation Window

When deriving a value from observed usage, choose an observation window long enough that the derived number is stable between evaluations — a window of about a day moves with normal load variation, so re-deriving on a schedule rewrites the same field back and forth instead of converging.
Where the metrics backend's retention caps how long that window can be, the stability has to come from the change threshold instead: act only on a gap wide enough to matter, and skip adjustments that fall within the measurement's own variation.
A time-based cooldown does not fix this; it only lengthens the oscillation period.

## Comparing Against the Manifest, Not the Pod

Decide whether a change is warranted by comparing the observation against the value the built manifest carries — read it from `kustomize build`, since the field may sit in a base rather than the overlay — not against the one the Pod is running: no agent here can commit, so an edit that has not reached the cluster would otherwise be re-derived on every evaluation.
A post-`OOMKilled` raise that the circularity clause below permits is the exception, because there the sampled peak is a lower bound rather than the peak — the kill proves the true peak passed the limit while the scrape missed it — so raise from the limit the container is actually running with rather than from the manifest value, which would otherwise double again on every evaluation.
Sustained CFS throttling is the same exception for `limits.cpu`, since the quota caps what the container can consume: anchor that raise on the limit the throttled Pod is running with as well.
Floor the result at the limit already in the manifest so that an undeployed raise is not undone.

## The Circularity Clause

Keep `requests.memory` at or above the steady-state plateau so the pod is not promoted in kubelet's eviction ordering, which ranks pods over their request by how far they exceed it — a request set to the idle floor makes a workload that legitimately spikes the first candidate to be evicted.
Before changing `requests.memory`, `limits.memory`, `limits.cpu` or `requests.cpu`, check whether any container in the pod derives its own cache, heap or parallelism ceiling from that value — through `resourceFieldRef` (`requests.memory` for a cache size, or for `GOMEMLIMIT` on a VPA target whose `controlledValues` is `RequestsOnly`; `limits.memory` for `GOMEMLIMIT` otherwise, `limits.cpu` for `GOMAXPROCS` or for a proxy or worker count expanded into args), or without one, through a JVM flag such as `-XX:MaxRAMPercentage` or a runtime that reads the cgroup directly such as tokio's `available_parallelism()` (see `### Rust (tokio)` in `.claude/rules/cluster/manifests.md`), neither of which leaves a `resourceFieldRef` to find.
Where it does, the value *is* the ceiling, so changing it is a functional change, and observed usage is circular evidence — the peak was measured under the very ceiling being changed.
Run this check before either anchor exception above, so a ceiling the workload derives from the field decides the outcome first.
For a CPU value consumed through a divisor-less `resourceFieldRef` the ceiling is `ceil()` of it in whole cores (see `### Go` in `.claude/rules/cluster/manifests.md`), so a change that leaves that ceiling where it was is not circular on that account.
That reasoning does not carry to the other paths: `ceil()` is flat across `(n-1, n]` while a round-down runtime moves at `n` itself, so `1500m` to `2000m` holds one count at 2 while moving the other from 1 to 2.
On the CFS quota's own account such a change is still circular: a container already throttling reports a usage peak capped by the quota, so read its throttled-period share before changing `limits.cpu` in either direction and treat the peak as a lower bound, the same way a kill makes the memory peak one.
Lowering a value that is itself such a cache or heap ceiling stays off limits whatever usage shows, because a cache merely not yet full reads far below its ceiling without that being evidence the ceiling is too large; record it for a human instead of deriving a cut from a peak measured under it.
Raising it is the same trap in the other direction once usage has reached the ceiling, since the workload fills whatever it is handed and the raise relocates the kill rather than resolving it — the exception is a kill that came from a transient spike, with usage well below the ceiling — real headroom does absorb that.
A parallelism ceiling has no kill to relocate and no cache to fill, so withhold the change in either direction whenever the count moves: it resizes the workload's thread or proxy pool and with it its memory behavior.
Scope the check to the field being written: a ceiling taken from `limits.memory` does not make a `requests.memory` change circular, and one taken from `requests.memory` does not make a `limits.memory` change circular.

## Establishing Which Mechanism Applies

Establish which mechanism applies from `kustomize build` output or the running Pod spec, never from the overlay patch alone: the overlay usually carries only the number while the consuming env or args sit in a base.
Read the consuming script or args rather than assuming the units: the `divisor` decides whether a memory value arrives as bytes or as MiB, and a CPU value as whole cores or millicores.
Where the ceiling comes from a field VPA owns under `controlledValues: RequestsOnly`, `minAllowed` for that resource is the floor of the ceiling and the only lever left, so keep it above the observed steady-state working set — a recommendation that falls to the floor otherwise drags the ceiling below the live set.
Read the live recommendation first: usage is circular evidence only while the recommendation actually sits at the floor.

## Example

Copy from: `cluster/manifests/utilities/redis/` (no `divisor`, so bytes), `cluster/manifests/utilities/varnish/` and `cluster/manifests/utilities/memcached/` (`divisor: 1Mi`), `cluster/manifests/utilities/minio/` (`GOMEMLIMIT` from `requests.memory`, floor in each consumer's `minio/patches/vertical_pod_autoscaler.yaml`)
