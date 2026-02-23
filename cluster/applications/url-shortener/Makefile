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
