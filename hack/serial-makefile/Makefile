# Disable jobserver mode
MAKEFLAGS := -j 1

ENTRYPOINT := $(dir $(abspath $(firstword $(MAKEFILE_LIST))))

.PHONY: lint
lint:
	@echo lint
	@cargo clippy -q --fix # `cargo clippy --fix` replaces `cargo check` and `cargo fix`

.PHONY: dev
dev:
	@cargo watch -c -w $(ENTRYPOINT)../../packages -N -s "mold --run cargo build" -s "command -v notify-send > /dev/null && notify-send 'Build completed' 'Cargo Watch has executed cargo build.'"
