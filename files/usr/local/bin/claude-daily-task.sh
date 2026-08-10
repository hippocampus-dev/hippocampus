#!/usr/bin/env -S bash -l

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

cd /opt/hippocampus

today=$(date +%Y-%m-%d)
yesterday=$(date -d yesterday +%Y-%m-%d)
yesterday_epoch=$(date -d "${yesterday} 00:00:00" +%s)
today_epoch=$(date -d "${today} 00:00:00" +%s)

log_format=$(cat <<EOS
After completing the work (regardless of whether any files were modified), always write a log in English to the Log path shown below. The Log path is an absolute path under your home directory; never write under the repository mirror files/home/kai/brain, always use the literal absolute path shown. Create parent directories as needed. Use this format:

## Investigated
<!-- What was checked and how -->

## Decided
<!-- Findings, root causes, and reasoning for each action or non-action -->

## Changed
<!-- Files modified with absolute paths and why. Write "No changes" if nothing was modified. -->

## Skipped
<!-- Issues intentionally not addressed and the reason (e.g., criteria not met, out of scope) -->
EOS
)

pids=()

claudex --print --dangerously-skip-permissions --remote-control --model=sonnet \
  -p "$(cat <<EOS
Investigate all pods on minikube across all namespaces. If any are in a non-running state, have high restart counts, or have containers not ready, fix the root cause. Do not change any setting that sizes a container's memory, including resources.requests.memory, resources.limits.memory, GOMEMLIMIT, GOGC, JVM heap options and the istio proxy resource annotations. Record the need in your log instead: separate tasks own part of the plain memory requests and limits, and everything they do not reach is deliberately left to a human. CPU is yours to fix as usual, except on a container that derives a thread or proxy count from limits.cpu, whether through a divisor-less resourceFieldRef as GOMAXPROCS and mcrouter's --num-proxies do or by reading the cgroup quota directly as a JVM or a tokio runtime does. There leave limits.cpu alone whatever value you would give it, and record what you would have set, since those runtimes round the quota in different directions and a change that leaves one count where it was moves another — 1500m to 2000m holds a rounded-up count at 2 while moving a rounded-down one from 1 to 2 — and that shifts the workload's parallelism and with it memory behavior a separate task owns. Settle which containers those are from the workload's language the way .claude/rules/cluster/manifests.md says to, and where that cannot be settled leave the value alone and record it rather than taking the absence of an answer for a no. Make minimal changes, do not refactor unrelated code.

${log_format}

Log path: ${HOME}/brain/report/${today}/tasks/pods.md
EOS
)" &
pids+=($!)

claudex --print --dangerously-skip-permissions --remote-control --model=sonnet \
  -p "$(cat <<EOS
Investigate all ArgoCD applications. If any are in a failed or degraded state, fix the root cause. Do not change any setting that sizes a container's memory, including resources.requests.memory, resources.limits.memory, GOMEMLIMIT, GOGC, JVM heap options and the istio proxy resource annotations. Record the need in your log instead: separate tasks own part of the plain memory requests and limits, and everything they do not reach is deliberately left to a human. CPU is yours to fix as usual, except on a container that derives a thread or proxy count from limits.cpu, whether through a divisor-less resourceFieldRef as GOMAXPROCS and mcrouter's --num-proxies do or by reading the cgroup quota directly as a JVM or a tokio runtime does. There leave limits.cpu alone whatever value you would give it, and record what you would have set, since those runtimes round the quota in different directions and a change that leaves one count where it was moves another — 1500m to 2000m holds a rounded-up count at 2 while moving a rounded-down one from 1 to 2 — and that shifts the workload's parallelism and with it memory behavior a separate task owns. Settle which containers those are from the workload's language the way .claude/rules/cluster/manifests.md says to, and where that cannot be settled leave the value alone and record it rather than taking the absence of an answer for a no. Make minimal changes, do not refactor unrelated code.

${log_format}

Log path: ${HOME}/brain/report/${today}/tasks/argocd.md
EOS
)" &
pids+=($!)

claudex --print --dangerously-skip-permissions --remote-control --model=sonnet \
  -p "$(cat <<EOS
Investigate recent GitHub Actions workflow runs. If any have failed, fix the root cause. Make minimal changes, do not refactor unrelated code.

${log_format}

Log path: ${HOME}/brain/report/${today}/tasks/gha.md
EOS
)" &
pids+=($!)

# All three edit resources: blocks in the same overlay files, so run them in sequence rather than concurrently.
# An earlier one must not gate a later one under set -e, so the status is carried and re-raised at the end.
(
sizing_code=0
claudex --print --dangerously-skip-permissions --remote-control --model=sonnet \
  -p "$(cat <<EOS
Reclaim over-provisioned memory requests on minikube. Enumerate Deployment, StatefulSet and DaemonSet containers across all namespaces that already have resources.requests.memory set, drop any workload managed by VPA and any injected sidecar such as istio-proxy, and record every drop in the log. Record likewise, without changing anything, every container whose Pod does not trace back to one of those three kinds, counting a Deployment's Pod as tracing back through its ReplicaSet, and including Pods with no owner at all, and say in each case why it is out of reach: an operator-managed CR such as a StrimziPodSet, a VitessShard or an EtcdLockserver, a static Pod defined on the node, or a Job.
Read usage from Mimir through Grafana's datasource proxy as .claude/skills/kubernetes-operations/reference/queries.md describes. Call M that container's max_over_time(container_memory_working_set_bytes{container!=""}[2d]) across the pods of its workload. Record and skip any container the query returns no series for.
Lower resources.requests.memory only when the manifest value is at least 256Mi, at least 2 times M, and the reduction is at least 128Mi. Set it to M multiplied by 1.2, rounded up to a whole Mi even where the manifest writes Gi, because rounding up in Gi can land back on the value already there and the cut silently never happens.
Never lower a container whose own cache or heap ceiling is derived from requests.memory, and record it instead: there the request is the ceiling, so M was measured under the very value you would cut. Decide that from the running Pod spec or from kustomize build, since the overlay usually carries only the number while the args consuming it live in a base.
Record and skip any container that was OOMKilled inside the same 2d window M covers, read from container.lastState.terminated.reason on that container with container.lastState.terminated.finishedAt for the time, because a kill anywhere in that window means M understates a peak the kill proves passed the limit. Say in the log which of three cases it is: a kill inside the past 24h on a container with its own resources.limits.memory, which a separate task considers unless that container sits outside its reach, a Knative Service among them, and whose own guards may still decline; a kill older than that, which no task acts on because that task only looks back 24h; or a kill on a container with no limit of its own, which came from node pressure and which nothing here can address. The last two are a human's to pick up, as is the first wherever that task cannot reach it.
Leave resources.limits, every CPU value, and every other memory-sizing setting alone: GOMEMLIMIT, GOGC, JVM heap options and the istio proxy resource annotations are a human's to change, not yours.
Confine edits to cluster/manifests/*/overlays/dev/ and follow .claude/rules/cluster/manifests.md, which loads for those files and governs circular ceilings, which value to compare against, and when to record a hypothesis instead of applying it. Record and leave alone anything whose value sits in a base manifest or in an operator-managed CR, and record and leave a Knative Service alone as well: its resources sit inline in the ksvc rather than in a Deployment patch, and this task does not size those. Never edit a live object, this script, or any rule.
Decide each container from these rules alone, on this run only.
Make minimal changes, do not refactor unrelated code.

${log_format}

Log path: ${HOME}/brain/report/${today}/tasks/memory-requests.md
EOS
)" || sizing_code=$?

claudex --print --dangerously-skip-permissions --remote-control --model=sonnet \
  -p "$(cat <<EOS
Relieve memory limits on minikube containers that were OOMKilled. Enumerate Deployment, StatefulSet and DaemonSet containers across all namespaces that have resources.limits.memory set and whose container.lastState.terminated.reason is OOMKilled with container.lastState.terminated.finishedAt inside the past 24h, read on that container rather than anywhere else in the Pod. Drop any workload managed by VPA and any injected sidecar such as istio-proxy, and record every drop in the log. Record likewise, without changing anything, every container whose Pod does not trace back to one of those three kinds, counting a Deployment's Pod as tracing back through its ReplicaSet, and including Pods with no owner at all, and say in each case why it is out of reach: an operator-managed CR such as a StrimziPodSet, a VitessShard or an EtcdLockserver, a static Pod defined on the node, or a Job.
Raise resources.limits.memory to twice that container's own limit on the Pod that was killed, taking the largest when several of its pods were killed with different limits, rounded up to a whole Mi even where the manifest writes Gi, and never below the limit the manifest already carries. Derive it from that limit rather than from observed usage: the kill proves the true peak passed the limit while a working set scraped once a minute missed it.
Change nothing and record it when the result would pass 2Gi, when the result equals what the manifest already carries, or when a comment on that container's entry in the overlay file you are about to edit, or inside that entry's resources block, says the limit deliberately caps a workload that grows without bound — an "Override upstream defaults" comment recording what upstream shipped is not that — since raising it there only widens the leak's runway. Read that comment from the overlay file itself; it does not survive kustomize build and never appears on a live object.
When the container takes its own heap or cache ceiling from limits.memory, raising the limit moves that ceiling too, so raise only while its usage stays below 80 percent of the very limit you anchored on. Decide that from the running Pod spec or from kustomize build, since the overlay usually carries only the number while the env consuming it lives in a base. Read that usage from Mimir through Grafana's datasource proxy as .claude/skills/kubernetes-operations/reference/queries.md describes, as max_over_time(container_memory_working_set_bytes{container!=""}[2d]) across the pods of its workload. At or above that share the application is filling the ceiling it was handed, so doubling only moves the same kill to a larger number: change nothing and record it. Do the same when the query returns no series for it.
Leave resources.requests.memory, every CPU value, and every other memory-sizing setting alone: GOMEMLIMIT, GOGC, JVM heap options and the istio proxy resource annotations are a human's to change, not yours, even when raising the limit moves what they resolve to.
Confine edits to cluster/manifests/*/overlays/dev/ and follow .claude/rules/cluster/manifests.md, which loads for those files and governs circular ceilings, which value to compare against, and when to record a hypothesis instead of applying it. Record and leave alone anything whose value sits in a base manifest or in an operator-managed CR, and record and leave a Knative Service alone as well: its resources sit inline in the ksvc rather than in a Deployment patch, and this task does not size those. Never edit a live object, this script, or any rule.
Decide each container from these rules alone, on this run only.
Make minimal changes, do not refactor unrelated code.

${log_format}

Log path: ${HOME}/brain/report/${today}/tasks/memory-limits.md
EOS
)" || sizing_code=$?

claudex --print --dangerously-skip-permissions --remote-control --model=sonnet \
  -p "$(cat <<EOS
Relieve CPU throttling on minikube containers. Enumerate Deployment, StatefulSet and DaemonSet containers across all namespaces that have resources.limits.cpu set, reading them from the running Pods rather than from the workload spec, since an injected sidecar is added at admission and appears only there. Drop any workload managed by VPA and any injected sidecar such as istio-proxy, and record every drop in the log — a sidecar takes its limit from the sidecar.istio.io/proxyCPULimit annotation rather than from a resources block, so where one crosses the threshold below, say so in the log and leave it for a human. Record likewise, without changing anything, every container whose Pod does not trace back to one of those three kinds, counting a Deployment's Pod as tracing back through its ReplicaSet, and including Pods with no owner at all, and say in each case why it is out of reach: an operator-managed CR such as a StrimziPodSet, a VitessShard or an EtcdLockserver, a static Pod defined on the node, or a Job.
Read throttling from Mimir through Grafana's datasource proxy as .claude/skills/kubernetes-operations/reference/queries.md describes. Call T that container's increase(container_cpu_cfs_throttled_periods_total{container!=""}[2d]) divided by its increase(container_cpu_cfs_periods_total{container!=""}[2d]), forming that ratio per pod and taking the largest across the pods of its workload. Record and skip any container either query returns no series for, and any whose period count over the window is zero, since the share is then undefined rather than low.
Raise resources.limits.cpu only when T is at least 5 percent, to twice that container's own limit on the Pod that was throttled, taking the largest when several of its pods run with different limits, and never below the value the manifest already carries. Derive it from that limit rather than from observed CPU usage: the quota caps what a throttled container can consume, so its usage peak is a lower bound rather than the peak.
Change nothing and record it when the result would pass 2000m, when the result equals what the manifest already carries, or when the container derives a thread or proxy count from limits.cpu, whether through a divisor-less resourceFieldRef or without one as a runtime reading the cgroup directly does, and ceil() of the value in whole cores would differ before and after — this task only ever doubles, and doubling leaves ceil() where it was only at or below half a core, where every runtime's count is one either way — since that shifts the workload's parallelism and with it memory behavior a separate task owns. Establish that from the running Pod spec or from kustomize build, since the overlay usually carries only the number while the env or args consuming it live in a base, and where a runtime reads the cgroup directly it leaves nothing in either, so settle it from the workload's language the way .claude/rules/cluster/manifests.md says to, and where that cannot be settled change nothing and record it rather than taking the absence of an answer for a no.
Never lower resources.limits.cpu and never touch resources.requests.cpu: only the request is scheduled against, so a limit above what a container uses costs nothing while the node has CPU to spare, and lowering one creates the throttling this task exists to relieve.
Leave every memory value alone: resources.requests.memory, resources.limits.memory, GOMEMLIMIT, GOGC, JVM heap options and the istio proxy resource annotations are other tasks' or a human's to change, not yours.
Confine edits to cluster/manifests/*/overlays/dev/ and follow .claude/rules/cluster/manifests.md, which loads for those files and governs circular ceilings, which value to compare against, and when to record a hypothesis instead of applying it. Record and leave alone anything whose value sits in a base manifest or in an operator-managed CR, and record and leave a Knative Service alone as well: its resources sit inline in the ksvc rather than in a Deployment patch, and this task does not size those. Never edit a live object, this script, or any rule.
Decide each container from these rules alone, on this run only.
Make minimal changes, do not refactor unrelated code.

${log_format}

Log path: ${HOME}/brain/report/${today}/tasks/cpu-limits.md
EOS
)" || sizing_code=$?
exit "$sizing_code"
) &
pids+=($!)

claudex --print --dangerously-skip-permissions --remote-control \
  -p "$(cat <<EOS
Generate a daily report for ${yesterday}. Gather information from:
1. That day's session logs under ~/.config/claudex/config/projects/ for the current project
2. That day's git log (git log --since="${yesterday}" --until="${today}" --all --oneline --numstat)
3. That day's Google Calendar events (gcal_list_events)
4. That day's task logs under ${HOME}/brain/report/${yesterday}/tasks/, written by the tasks of that day's own run - a day the timer skipped leaves no such directory, so record that it is missing and move on
5. That day's shell commands, which fish stamps with an epoch in its history (awk -v s=${yesterday_epoch} -v e=${today_epoch} '/^- cmd: /{c=substr(\$0,8)} /^  when: /{t=\$2+0; if (t>=s && t<e && c!="") {print c; c=""}}' ${HOME}/.local/share/fish/fish_history) - that file carries years of history, so never read it whole, and where a command carries a token, key or password as a literal, name the command without reproducing that value

Write the report in Japanese to the absolute path ${HOME}/brain/report/${yesterday}.md (never write under the repository mirror files/home/kai/brain) using this format:

# Daily Report - ${yesterday}

## Accomplished
<!-- From session logs, git commits, calendar events, task logs, and shell commands -->

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
