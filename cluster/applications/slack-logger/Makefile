.DEFAULT_GOAL := all

ATLAS_VERSION := latest
ATLAS_GOTOOLCHAIN := go1.24.13
SQLC_VERSION := v1.31.1

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

.PHONY: validate
validate:
	@GOTOOLCHAIN=$(ATLAS_GOTOOLCHAIN) go install ariga.io/atlas/cmd/atlas@$(ATLAS_VERSION)
	@$(shell go env GOPATH)/bin/atlas migrate validate --dir file://migrations
	@go install github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION)
	@$(shell go env GOPATH)/bin/sqlc diff

.PHONY: generate
generate:
	@GOTOOLCHAIN=$(ATLAS_GOTOOLCHAIN) go install ariga.io/atlas/cmd/atlas@$(ATLAS_VERSION)
	@$(shell go env GOPATH)/bin/atlas migrate diff --env local
	@$(shell go env GOPATH)/bin/atlas migrate hash --env local
	@go install github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION)
	@$(shell go env GOPATH)/bin/sqlc generate

.PHONY: migrate
migrate:
	@GOTOOLCHAIN=$(ATLAS_GOTOOLCHAIN) go install ariga.io/atlas/cmd/atlas@$(ATLAS_VERSION)
	@dotenvx run -- $(shell go env GOPATH)/bin/atlas migrate apply --env remote --allow-dirty

.PHONY: all
all: fmt lint tidy test
	@

.PHONY: dev
dev:
	@watchexec -N -c -rw $(ENTRYPOINT) --stop-signal SIGKILL go run *.go

.PHONY: FORCE
FORCE:
