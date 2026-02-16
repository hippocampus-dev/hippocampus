# ext-proc Makefile Pattern

Extends the Rust Makefile template with `e2e` target for Kind-based integration testing.

## Additional Targets

Add to the standard Rust Makefile template:

```makefile
.PHONY: e2e
e2e:
	@kind create cluster --name {app-name}
	@trap 'kind delete cluster --name {app-name}' EXIT ERR INT; ./e2e.sh
```

| Target | Purpose |
|--------|---------|
| `make e2e` | Create Kind cluster, run e2e tests, delete cluster on exit |
| `make dev` | `skaffold dev --port-forward` (Kubernetes deployment pattern) |

## e2e.sh Script Pattern

```bash
#!/usr/bin/env bash
set -eo pipefail

cd e2e && skaffold run && cd ..
kubectl port-forward svc/{app-name} 8080:8080 8081:8081 -n e2e-{app-name} > /dev/null 2>&1 &
trap "kill $! 2>/dev/null" EXIT
cat k6/index.js | docker compose exec -T k6 k6 run -
```

## Key Points

* Kind cluster lifecycle is managed by the Makefile `e2e` target via `trap ... EXIT ERR INT`
* Port-forward cleanup is managed by `e2e.sh` via `trap ... EXIT`
* `trap` is set after `&` so `$!` is expanded immediately via double quotes, capturing the exact PID
* `e2e/skaffold.yaml` uses `context: ..` to reference the parent Dockerfile
* k6 tests run via `docker compose exec` against the local k6 service
