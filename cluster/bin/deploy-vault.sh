#!/usr/bin/env bash

set -e

ENTRYPOINT=$(cd $(dirname ${BASH_SOURCE[0]}); pwd)

kubectl apply -k ${ENTRYPOINT}/../manifests/vault/overlays/dev
until kubectl -n vault wait --for=condition=Ready pod -l app.kubernetes.io/name=vault --timeout=10m; do sleep 1; done
