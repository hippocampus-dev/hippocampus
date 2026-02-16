#!/usr/bin/env bash
set -eo pipefail

cd e2e && skaffold run && cd ..
kubectl port-forward svc/envoy-request-hasher 8080:8080 8081:8081 -n e2e-envoy-request-hasher > /dev/null 2>&1 &
trap "kill $! 2>/dev/null" EXIT
cat k6/index.js | docker compose exec -T k6 k6 run -
