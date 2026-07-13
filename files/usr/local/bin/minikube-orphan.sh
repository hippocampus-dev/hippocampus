#!/usr/bin/env bash

set -eo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

while sleep 60; do
  minikube ssh -- sudo bash -s <<'REMOTE'
set -eo pipefail
known=$(ctr -n k8s.io containers ls -q)
if [ -z "$known" ]; then
  exit 0
fi
ps -eo pid,args | awk -v known="$known" '
  BEGIN {
    n = split(known, arr, "\n")
    for (i=1; i<=n; i++) known_ids[arr[i]] = 1
  }
  /containerd-shim-runc-v2/ {
    id = ""
    for (i=1; i<=NF; i++) if ($i == "-id") { id=$(i+1); break }
    if (id != "" && !(id in known_ids)) print $1
  }
' | xargs -r kill -9
REMOTE
done
