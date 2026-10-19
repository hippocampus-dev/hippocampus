.DEFAULT_GOAL := dev

ENTRYPOINT := $(dir $(abspath $(firstword $(MAKEFILE_LIST))))

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
	@uv run -- playwright install chromium

.PHONY: dev
dev: install
	@watchexec -N -c -rw $(ENTRYPOINT) --stop-signal SIGKILL uv run -- python main.py

.PHONY: FORCE
FORCE:
