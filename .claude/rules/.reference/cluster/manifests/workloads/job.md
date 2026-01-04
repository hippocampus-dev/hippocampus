# Job Workload

One-time or per-sync tasks managed by ArgoCD.

## Pattern Selection

| Condition | Pattern | Example |
|-----------|---------|---------|
| Idempotent initialization (bucket creation) | Replace | `cluster/manifests/assets/overlays/dev/job.yaml` |
| Must run every sync (migrations, verification) | Hook | `cluster/manifests/adhoc/overlays/dev/job.yaml` |

## Replace Pattern

Runs when Job definition changes. Uses `kubectl replace` internally.

```yaml
metadata:
  annotations:
    argocd.argoproj.io/sync-wave: "-1"
    argocd.argoproj.io/sync-options: Replace=true
spec:
  completions: 1
  parallelism: 1
  backoffLimit: 6
  podFailurePolicy:
    rules:
      - action: FailJob
        onExitCodes:
          operator: In
          values: [127]
      - action: Ignore
        onPodConditions:
          - type: DisruptionTarget
            status: "True"
```

Do NOT use `completionMode: Indexed` - `kubectl replace` fails on auto-generated selector mismatch.

## Hook Pattern

Runs before (`PreSync`) or after (`PostSync`) every sync. Old Job deleted first by `BeforeHookCreation`.

```yaml
metadata:
  annotations:
    argocd.argoproj.io/hook: PostSync
    argocd.argoproj.io/hook-delete-policy: BeforeHookCreation
spec:
  completions: 1
  parallelism: 1
  completionMode: Indexed
  backoffLimitPerIndex: 6
  maxFailedIndexes: 1
  podReplacementPolicy: Failed
  podFailurePolicy:
    rules:
      - action: FailJob
        onExitCodes:
          operator: In
          values: [127]
      - action: Ignore
        onPodConditions:
          - type: DisruptionTarget
            status: "True"
```

`completionMode: Indexed` is safe here because the old Job is deleted before creation.
