#!/usr/bin/env bash
set -eo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

cd e2e && skaffold run && cd ..
kubectl wait --for=jsonpath='{.status.phase}'=Succeeded pod/fuse-csi-driver-e2e -n e2e-fuse-csi-driver --timeout=120s
