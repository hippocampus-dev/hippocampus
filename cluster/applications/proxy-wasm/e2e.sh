#!/usr/bin/env bash
set -eo pipefail

skaffold run
kubectl port-forward svc/skaffold-proxy-wasm 8080:8080 8081:8081 -n skaffold-proxy-wasm > /dev/null 2>&1 &
trap "kill $! 2>/dev/null" EXIT
cat k6/fallback-filter/index.js | docker compose exec -T k6 k6 run -
cat k6/cookie-manipulator/request.js | docker compose exec -T k6 k6 run -
cat k6/cookie-manipulator/response.js | docker compose exec -T k6 k6 run -
