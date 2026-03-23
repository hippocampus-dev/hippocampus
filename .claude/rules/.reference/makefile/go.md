# Go Makefile Pattern

## Pattern Selection

| Condition | Pattern | Example |
|-----------|---------|---------|
| Has `go.mod` | Standard (below) | `cluster/applications/exactly-one-pod-hook/` |
| Has `go.mod` + CRD | CRD Controller | `cluster/applications/github-actions-runner-controller/` |
| No `go.mod` (single file, stdlib only) | Standalone | `cluster/applications/bakery/` |

## Standard Template (with go.mod)

```makefile
.DEFAULT_GOAL := all

.PHONY: fmt
fmt:
	@go fmt ./...
	@go install golang.org/x/tools/cmd/goimports@latest
	@$(shell go env GOPATH)/bin/goimports -w .

.PHONY: lint
lint:
	@go vet ./...

.PHONY: tidy
tidy:
	@go mod tidy

.PHONY: test
test:
	@go test -race -bench=. -benchmem -trimpath ./...

.PHONY: all
all: fmt lint tidy test
	@

.PHONY: dev
dev:
	@skaffold dev --port-forward
```

## CRD Controller Template

For controllers with Custom Resource Definitions, add `gen` target:

```makefile
.DEFAULT_GOAL := all

.PHONY: fmt
fmt:
	@go fmt ./...
	@go install golang.org/x/tools/cmd/goimports@latest
	@$(shell go env GOPATH)/bin/goimports -w .

.PHONY: lint
lint:
	@go vet ./...

.PHONY: tidy
tidy:
	@go mod tidy

.PHONY: test
test:
	@go test -race -bench=. -benchmem -trimpath ./...

.PHONY: all
all: gen fmt lint tidy test
	@

.PHONY: gen
gen:
	@go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.19.0
	@$(shell go env GOPATH)/bin/controller-gen paths="./..." object crd:crdVersions=v1 output:crd:artifacts:config=manifests/crd

.PHONY: dev
dev:
	@kind create cluster --name {app-name} --config kind.yaml
	@trap 'kind delete cluster --name {app-name}' EXIT ERR INT; skaffold dev --port-forward

.PHONY: FORCE
FORCE:
```

| Target | Purpose |
|--------|---------|
| `gen` | Generate CRD manifests and DeepCopy methods |
| `all` | Includes `gen` as first dependency |
| `dev` | Creates Kind cluster for local testing |

## Standalone Template (no go.mod)

For single-file applications using only stdlib:

```makefile
.DEFAULT_GOAL := all

ENTRYPOINT := $(dir $(abspath $(firstword $(MAKEFILE_LIST))))

.PHONY: fmt
fmt:
	@go fmt *.go

.PHONY: lint
lint:
	@go vet *.go

.PHONY: test
test:
	@go test -race -bench=. -benchmem -trimpath *.go

.PHONY: all
all: fmt lint test
	@

.PHONY: dev
dev:
	@watchexec -N -c -rw $(ENTRYPOINT) --stop-signal SIGKILL go run *.go

.PHONY: FORCE
FORCE:
```

## dev Target Variations

The `dev` target depends on runtime requirements, not `go.mod` presence:

| Condition | dev Target | Example |
|-----------|------------|---------|
| K8s webhook/controller with skaffold.yaml | `skaffold dev --port-forward` | `exactly-one-pod-hook/` |
| Standalone (can run without K8s) | `watchexec ... go run` | `github-token-server/`, `http-kvs/` |
| CRD controller (has `api/` directory) | `kind create cluster` + skaffold | `github-actions-runner-controller/` |

Check if `api/` directory or `kind.yaml` exists to determine which pattern to use.

## Key Points

| Pattern | fmt | tidy | gen |
|---------|-----|------|-----|
| Standard | `./...` + goimports | Required | Not applicable |
| CRD Controller | `./...` + goimports | Required | Required (first in `all`) |
| Standalone | `*.go` only | Not applicable | Not applicable |
