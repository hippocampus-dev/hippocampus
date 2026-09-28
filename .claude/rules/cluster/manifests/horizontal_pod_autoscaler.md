---
paths:
  - "**/horizontal_pod_autoscaler.yaml"
---

* Write a CPU metric as `type: ContainerResource` with `containerResource.container`, never the pod-aggregate `type: Resource` - the aggregate counts the admission-injected `istio-proxy`/`istio-validation` (see `### Istio Sidecar Injection` in `.claude/rules/cluster/manifests.md`) in both the usage and the request it divides by, so a sidecar CPU spike on its own holds the HPA over target and cycles replicas while `kustomize build`, `kubectl apply --dry-run=server` and ArgoCD's Sync Status all stay green
* Name the container that measurement shows carries the workload's load, not the one sharing the workload's name - read per-container usage as `## PromQL (Mimir)` in `.claude/skills/kubernetes-operations/reference/queries.md` describes, since a pod's helper container can outrun the application container it sits beside
* Give the named container its own `resources.requests.cpu` - `type: Utilization` divides by that container's request alone and does not fall back to the pod total, so the HPA leaves the replica count where it is when the container has none
