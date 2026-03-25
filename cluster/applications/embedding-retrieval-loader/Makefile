.DEFAULT_GOAL := all

.PHONY: fmt
fmt:
	@uvx ruff format

.PHONY: lint
lint:
	@uvx ruff check --fix

.PHONY: all
all: fmt lint
	@

.PHONY: install
install:
	@uv sync --frozen

.PHONY: FORCE
FORCE:
