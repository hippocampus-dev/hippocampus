# Python Makefile Pattern

## Template

```makefile
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

.PHONY: dev
dev: install
	@watchexec -Nc -rw $(ENTRYPOINT) --stop-signal SIGKILL uv run -- python main.py
```

## Key Points

* Uses `uvx ruff` for formatting and linting
* `install` uses `uv sync --frozen` for reproducible installs
* `dev` uses `watchexec` for auto-reload
* `ENTRYPOINT` captures the Makefile directory for watchexec
