---
paths:
  - "cluster/manifests/prometheus/base/files/prometheus.yaml"
---

* When changing `scrape_interval` (global or per-job), update dependent components with stale-tolerance windows accordingly
* Close every new `scrape_configs` entry with the sharding pair - `hashmod` into `__tmp_hash` with `modulus: ${SHARD_COUNT}`, then `keep` on `regex: "${SHARD_ID}"` - hashing a label the job's own earlier relabels leave distinct per target (see `## Sharding`); `cluster/manifests/prometheus/base/stateful_set.yaml` renders one config per shard, so a job missing the pair is kept by every shard and ingests its series once per shard, while a job hashing a label those relabels have already collapsed to one value lands entirely on a single shard - neither shows up as a scrape error

## Sharding

| Job shape | `hashmod` `source_labels` |
|-----------|---------------------------|
| The job's relabels leave `__address__` distinct per target (`role: endpoints`, `role: pod`, `role: service`) | `__address__` |
| The job rewrites `__address__` to one API server proxy address (`role: node`) | `__meta_kubernetes_node_name` |
| The job rewrites `__address__` to one exporter address and carries the real target in `instance` (`static_configs` behind an exporter) | `instance` |

## Cross-Component Dependencies

| Field changed | Also update | Constraint |
|---------------|-------------|------------|
| `scrape_interval` (max across all jobs) | `querier.lookback_delta` in `cluster/manifests/mimir/overlays/dev/files/mimir.yaml` | `lookback_delta > 2 × max scrape_interval` (otherwise instant/range queries silently miss series after one failed scrape) |
