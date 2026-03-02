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
| base/ | cron_job.yaml | Schedule and job template |

## Key Modifications

- `kustomization.yaml`: Update image name and digest
- `cron_job.yaml`: Update schedule, container name, command
- `schedule`: Use cron syntax (e.g., `*/10 * * * *` for every 10 minutes)
