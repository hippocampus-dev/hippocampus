# CronJob Workload

Scheduled tasks that run periodically.

## When to Use

- Periodic maintenance tasks
- Scheduled reports, backups
- Regular cleanup jobs

## Example

MUST copy from: `cluster/manifests/descheduler/`

## Files

| Directory | File | Purpose |
|-----------|------|---------|
| base/ | cron_job.yaml | Job template (no schedule) |
| overlays/dev/patches/ | cron_job.yaml | Environment-specific schedule and patches |

## Key Modifications

- `kustomization.yaml`: Update image name and digest
- `base/cron_job.yaml`: Update container name, command (do NOT set `schedule` here)
- `overlays/dev/patches/cron_job.yaml`: Set `schedule` using cron syntax (e.g., `*/10 * * * *` for every 10 minutes)
