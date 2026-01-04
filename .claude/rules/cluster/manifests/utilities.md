---
paths:
  - "cluster/manifests/utilities/**/*.yaml"
---

* Development environment utilities (redis, minio, memcached, etc.) referenced by other manifests
* Flat structure (no base/overlays)
* Copy existing utility (e.g., `cluster/manifests/utilities/redis/`) as template
* Referenced from other manifests as `overlays/dev/redis/`, `overlays/dev/minio/`, etc.

## Files

| File | Purpose |
|------|---------|
| kustomization.yaml | Image configuration |
| stateful_set.yaml | StatefulSet workload |
| service.yaml | ClusterIP or Headless service |
| pod_disruption_budget.yaml | Availability during updates |
| files/ | Configuration files (optional) |
| horizontal_pod_autoscaler.yaml | HPA (optional) |
| network_policy.yaml | Network rules (optional) |
