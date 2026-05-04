#!/usr/bin/env -S bash -l

set -eo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

cd /opt/hippocampus

today=$(date +%Y-%m-%d)
yesterday=$(date -d yesterday +%Y-%m-%d)

log_format=$(cat <<EOS
After completing the work (regardless of whether any files were modified), always write a log in English to the Log path shown below. Create parent directories as needed. Use this format:

## Investigated
<!-- What was checked and how -->

## Decided
<!-- Findings, root causes, and reasoning for each action or non-action -->

## Changed
<!-- Files modified with absolute paths and why. Write "No changes" if nothing was modified. -->

## Skipped
<!-- Issues intentionally not addressed and the reason (e.g., 7-day rule, out of scope) -->
EOS
)

pids=()

claudex --print --dangerously-skip-permissions --remote-control --model=sonnet \
  -p "$(cat <<EOS
Investigate all pods on minikube across all namespaces. If any are in a non-running state, have high restart counts, or have containers not ready, fix the root cause. Make minimal changes, do not refactor unrelated code.

${log_format}

Log path: ~/brain/report/${today}/tasks/pods.md
EOS
)" &
pids+=($!)

claudex --print --dangerously-skip-permissions --remote-control --model=sonnet \
  -p "$(cat <<EOS
Investigate all ArgoCD applications. If any are in a failed or degraded state, fix the root cause. Make minimal changes, do not refactor unrelated code.

${log_format}

Log path: ~/brain/report/${today}/tasks/argocd.md
EOS
)" &
pids+=($!)

claudex --print --dangerously-skip-permissions --remote-control --model=sonnet \
  -p "$(cat <<EOS
Investigate recent GitHub Actions workflow runs. If any have failed, fix the root cause. Make minimal changes, do not refactor unrelated code.

${log_format}

Log path: ~/brain/report/${today}/tasks/gha.md
EOS
)" &
pids+=($!)

claudex --print --dangerously-skip-permissions --remote-control --model=sonnet \
  -p "$(cat <<EOS
Investigate resource requests and limits for all Deployment and StatefulSet containers on minikube across all namespaces.
Skip Job, CronJob, and any workload managed by VPA.
Compare actual usage over the past 1 day from Prometheus: use rate() for container_cpu_usage_seconds_total and avg_over_time() for container_memory_working_set_bytes, but for memory also check max_over_time() to account for spikes.
Do not rely on kubectl top as it only shows a point-in-time snapshot.
Before adjusting any workload, check git log for the past 7 days for the specific manifest file. If the same container in the same manifest file had its resource requests or limits adjusted within the past 7 days, skip it unless the workload is currently experiencing OOMKill, CrashLoopBackOff, or CPU throttling.
Because workloads tend to be over-provisioned, bias request adjustments toward the lower bound of the past 1 day observed usage, and only add headroom justified by memory spikes, OOMKill, CrashLoopBackOff, or CPU throttling.
For memory requests specifically, never set the new value below max_over_time(container_memory_working_set_bytes[1d]) * 1.2 to prevent oscillation between OOMKill recovery and aggressive downward adjustments.
If any are still significantly over-provisioned or under-provisioned, fix the manifests in this repository.
Do not add resources.requests to containers that do not already have resources.requests set.
Do not add resources.limits to containers that do not already have resources.limits set.
Make minimal changes, do not refactor unrelated code.

${log_format}

Log path: ~/brain/report/${today}/tasks/resources.md
EOS
)" &
pids+=($!)

claudex --print --dangerously-skip-permissions --remote-control \
  -p "$(cat <<EOS
Generate a daily report for ${yesterday}. Gather information from:
1. That day's session logs under ~/.config/claudex/config/projects/ for the current project
2. That day's git log (git log --since="${yesterday}" --until="${today}" --all --oneline)
3. That day's Google Calendar events (gcal_list_events)

Write the report in Japanese to ~/brain/report/${yesterday}.md using this format:

# Daily Report - ${yesterday}

## Accomplished
<!-- From session logs, git commits, and calendar events -->

## Decisions
<!-- Key decisions and their reasoning -->

## In Progress
<!-- Unfinished work, context for tomorrow -->
EOS
)" &
pids+=($!)

code=0
for pid in "${pids[@]}"; do
  wait "$pid" || code=$?
done
exit "$code"
