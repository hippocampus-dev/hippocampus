---
paths:
  - "cluster/manifests/**/*.yaml"
---

* Production manifests in `manifests/`, development in `skaffold/`
* Use singular resource type with underscores (`service_account.yaml`)
* Pin images by digest in kustomization.yaml, not by tag
* Mirror external images via `.github/workflows/99_mirroring.yaml` (do not reference external images directly)
* Only include explicitly required fields (do not add optional configurations unless requested)
* All containers require secure defaults (non-root UID 65532, no privilege escalation, read-only filesystem)
* All workloads require standard labels (`app.kubernetes.io/name`, `app.kubernetes.io/component`)
* When avoiding hardcoded values: use Kustomize (replacements, configMapGenerator) for cross-resource references, Downward API for own pod metadata (name, labels, resources), scripts/API calls as last resort (example: `cluster/manifests/utilities/redis/` and `cluster/manifests/translator/overlays/dev/redis/kustomization.yaml`)

## Deploying External Projects

When given a GitHub URL:

1. Open the URL to understand the project type
2. Determine workload type from table below
3. Copy manifests from the Example in the workload reference
4. Mirror Docker images via `.github/workflows/99_mirroring.yaml`
5. Create ArgoCD Application

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

If creating a workload, choose the appropriate pattern:
  Read: `.claude/rules/.reference/cluster/manifests/workloads/external-service.md`
  Read: `.claude/rules/.reference/cluster/manifests/workloads/internal-service.md`
  Read: `.claude/rules/.reference/cluster/manifests/workloads/stateful.md`
  Read: `.claude/rules/.reference/cluster/manifests/workloads/daemon.md`
  Read: `.claude/rules/.reference/cluster/manifests/workloads/cronjob.md`
  Read: `.claude/rules/.reference/cluster/manifests/workloads/job.md`
  Read: `.claude/rules/.reference/cluster/manifests/workloads/knative-service.md`
  Read: `.claude/rules/.reference/cluster/manifests/workloads/knative-eventing.md`
