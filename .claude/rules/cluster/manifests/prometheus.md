---
paths:
  - "cluster/manifests/prometheus/base/files/prometheus.yaml"
---

* When changing `scrape_interval` (global or per-job), update dependent components with stale-tolerance windows accordingly

## Cross-Component Dependencies

| Field changed | Also update | Constraint |
|---------------|-------------|------------|
| `scrape_interval` (max across all jobs) | `querier.lookback_delta` in `cluster/manifests/mimir/overlays/dev/files/mimir.yaml` | `lookback_delta > 2 × max scrape_interval` (otherwise instant/range queries silently miss series after one failed scrape) |
