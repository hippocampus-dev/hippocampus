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

.PHONY: build
build:
	@npm run build

.PHONY: all
all: fmt lint install build
	@

.PHONY: dev
dev: install
	@npm run dev

.PHONY: FORCE
FORCE:
