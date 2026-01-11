# Go Makefile Pattern

## Pattern Selection

| Condition | Pattern | Example |
|-----------|---------|---------|
| Has `go.mod` | Standard (below) | `cluster/applications/exactly-one-pod-hook/` |
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
	@watchexec -Nc -rw $(ENTRYPOINT) --stop-signal SIGKILL go run *.go

.PHONY: FORCE
FORCE:
```

## dev Target Variations

The `dev` target depends on runtime requirements, not `go.mod` presence:

| Condition | dev Target | Example |
|-----------|------------|---------|
| K8s webhook/controller with skaffold.yaml | `skaffold dev --port-forward` | `exactly-one-pod-hook/` |
| Standalone (can run without K8s) | `watchexec ... go run` | `github-token-server/`, `http-kvs/` |
| Controller needing own K8s cluster | `kind create cluster` + skaffold | `github-actions-runner-controller/` |

Check if `skaffold.yaml` exists to determine which pattern to use.

## Key Points

| Pattern | fmt | tidy |
|---------|-----|------|
| Standard | `./...` + goimports | Required |
| Standalone | `*.go` only | Not applicable |
