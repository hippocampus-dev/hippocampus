#!/usr/bin/env -S bash -l

set -eo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

cd /opt/hippocampus

pids=()

claudex --print --dangerously-skip-permissions --remote-control \
  -p "$(cat <<EOS
Investigate all pods on minikube across all namespaces. If any are in a non-running state, have high restart counts, or have containers not ready, fix the root cause. Make minimal changes, do not refactor unrelated code.
EOS
)" &
pids+=($!)

claudex --print --dangerously-skip-permissions --remote-control \
  -p "$(cat <<EOS
Investigate all ArgoCD applications. If any are in a failed or degraded state, fix the root cause. Make minimal changes, do not refactor unrelated code.
EOS
)" &
pids+=($!)

claudex --print --dangerously-skip-permissions --remote-control \
  -p "$(cat <<EOS
Investigate recent GitHub Actions workflow runs. If any have failed, fix the root cause. Make minimal changes, do not refactor unrelated code.
EOS
)" &
pids+=($!)

claudex --print --dangerously-skip-permissions --remote-control \
  -p "$(cat <<EOS
Investigate resource requests and limits for all containers on minikube across all namespaces. Compare actual usage against configured requests/limits. If any are significantly over-provisioned or under-provisioned, fix the manifests in this repository. Make minimal changes, do not refactor unrelated code.
EOS
)" &
pids+=($!)

claudex --print --dangerously-skip-permissions --remote-control \
  -p "$(cat <<EOS
Generate a daily report for $(date +%Y-%m-%d). Gather information from:
1. Today's session logs under ~/.config/claudex/config/projects/ for the current project
2. Today's git log (git log --since="$(date +%Y-%m-%d)" --all --oneline)
3. Today's Google Calendar events (gcal_list_events)

Write the report in markdown to ~/brain/report/$(date +%Y-%m-%d).md using this format:

---
# Daily Report - $(date +%Y-%m-%d)

## Accomplished
<!-- From session logs, git commits, and calendar events -->

## Decisions
<!-- Key decisions and their reasoning -->

## In Progress
<!-- Unfinished work, context for tomorrow -->

---
Be concise.
EOS
)" &
pids+=($!)

code=0
for pid in "${pids[@]}"; do
  wait "$pid" || code=$?
done
exit "$code"
