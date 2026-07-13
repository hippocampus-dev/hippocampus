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
	@kind create cluster --name fuse-csi-driver --config kind.yaml
	@trap 'kind delete cluster --name fuse-csi-driver' EXIT ERR INT; skaffold dev --port-forward

.PHONY: e2e
e2e:
	@kind create cluster --name fuse-csi-driver --config kind.yaml
	@trap 'kind delete cluster --name fuse-csi-driver' EXIT ERR INT; ./e2e.sh

.PHONY: FORCE
FORCE:
