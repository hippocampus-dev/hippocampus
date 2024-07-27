#!/usr/bin/env bash

set -eo pipefail

kubectl apply -k manifests/vault/overlays/dev
until kubectl -n vault wait --for=condition=Ready pod -l app.kubernetes.io/name=vault --timeout=10m; do sleep 1; done
