.DEFAULT_GOAL := test

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

.PHONY: dev
test: install
	@uv run -- python -m unittest discover -s tests

.PHONY: FORCE
FORCE:
