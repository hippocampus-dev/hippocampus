#!/usr/bin/env -S bash -l

set -eo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

cd /opt/hippocampus

claudex --print \
  --allowedTools "Bash,Read,Write,Edit,Glob,Grep,Agent,WebFetch" \
  -p "Investigate all pods on minikube across all namespaces. If anomalies are found, fix the manifests and create a PR. If the root cause is unclear, create a GitHub issue instead. Manifests are in cluster/manifests/<app>/. Make minimal changes, do not refactor unrelated code."
