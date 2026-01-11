.DEFAULT_GOAL := all

ENTRYPOINT := $(dir $(abspath $(firstword $(MAKEFILE_LIST))))

.PHONY: fmt
fmt:
	@cargo fmt

.PHONY: lint
lint:
	@cargo clippy --fix --allow-dirty

.PHONY: test
test:
	@cargo test

.PHONY: build
build:
	@cargo build --release

.PHONY: all
all: fmt lint test build
	@

.PHONY: dev
dev:
	@watchexec -Nc -rw $(ENTRYPOINT) --stop-signal SIGKILL cargo run
