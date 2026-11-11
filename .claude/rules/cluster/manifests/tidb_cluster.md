---
paths:
  - "**/tidb_cluster.yaml"
---

* PD copies `[replication]` and `[schedule]` into its own etcd when the cluster is bootstrapped and reads them from there afterwards, so an edit to that block in `pd.config` reaches only a cluster bootstrapped later while the running one keeps its old value - apply the same value with `pd-ctl config set` alongside the CR edit, since `kustomize build` passes and ArgoCD reports Synced either way
