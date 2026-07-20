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
all: gen fmt lint tidy test
	@

.PHONY: gen
gen:
	@go install github.com/ogen-go/ogen/cmd/ogen@latest
	@go generate ./...

.PHONY: dev
dev:
	@watchexec -N -c -rw $(ENTRYPOINT) --stop-signal SIGKILL go run *.go

.PHONY: FORCE
FORCE:
