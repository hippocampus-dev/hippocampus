.DEFAULT_GOAL := all

.PHONY: fmt
fmt:
	@npm run fmt

.PHONY: lint
lint:
	@npm run lint

.PHONY: install
install:
	@npm ci

.PHONY: test
test: install
	@npm run test

.PHONY: all
all: fmt lint test
	@

.PHONY: FORCE
FORCE:
