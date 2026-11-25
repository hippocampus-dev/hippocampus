.DEFAULT_GOAL := all

.PHONY: fmt
fmt:
	@go fmt ./...
	@go install golang.org/x/tools/cmd/goimports@latest
	@$(shell go env GOPATH)/bin/goimports -w .

.PHONY: lint
lint:
	@go vet -tags e2e ./...

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

.PHONY: e2e
e2e:
	@./e2e.sh

.PHONY: FORCE
FORCE:
