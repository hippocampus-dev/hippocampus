# Disable jobserver mode
MAKEFLAGS := -j 1

ENTRYPOINT := $(dir $(abspath $(firstword $(MAKEFILE_LIST))))

.PHONY: lint
lint:
	@cargo clippy -q --fix # `cargo clippy --fix` replaces `cargo check` and `cargo fix`

.PHONY: dev
dev:
	@watchexec -Nc -rw $(ENTRYPOINT)/../../packages --stop-signal SIGKILL mold --run cargo build

.PHONY: watch-decrypt
watch-decrypt:
	@watchexec -c -e enc bash $(ENTRYPOINT)/../../bin/decrypt.sh

.PHONY: FORCE
FORCE:
