.DEFAULT_GOAL := all

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
	@go run *.go

.PHONY: FORCE
FORCE:
