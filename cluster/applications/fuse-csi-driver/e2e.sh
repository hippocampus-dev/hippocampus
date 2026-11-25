#!/usr/bin/env bash
set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

cd e2e
# Skaffold reports the native sidecar's exit code 143 as a deployment failure.
skaffold run --status-check=false
cd ..
kubectl wait --for=jsonpath='{.status.phase}'=Succeeded pod/fuse-csi-driver-e2e -n e2e-fuse-csi-driver --timeout=120s
