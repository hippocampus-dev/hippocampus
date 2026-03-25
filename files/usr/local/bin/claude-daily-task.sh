#!/usr/bin/env -S bash -l

set -eo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

cd /opt/hippocampus

claudex --print --dangerously-skip-permissions \
  -p "$(cat <<EOS
Investigate all pods on minikube across all namespaces. If anomalies are found, fix the root cause. Make minimal changes, do not refactor unrelated code.
EOS
)"

claudex --print --dangerously-skip-permissions \
  -p "$(cat <<EOS
Investigate all ArgoCD applications. If any are in a failed or degraded state, fix the root cause. Make minimal changes, do not refactor unrelated code.
EOS
)"

claudex --print --dangerously-skip-permissions \
  -p "$(cat <<EOS
Investigate recent GitHub Actions workflow runs. If any have failed, fix the root cause. Make minimal changes, do not refactor unrelated code.
EOS
)"
