.DEFAULT_GOAL := all

ENTRYPOINT := $(dir $(abspath $(firstword $(MAKEFILE_LIST))))

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
	@watchexec -Nc -rw $(ENTRYPOINT) --stop-signal SIGKILL go run *.go --local-address=127.0.0.1:8080 --remote-address=127.0.0.1:6379

.PHONY: FORCE
FORCE:
