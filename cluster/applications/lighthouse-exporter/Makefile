.DEFAULT_GOAL := dev

ENTRYPOINT := $(dir $(abspath $(firstword $(MAKEFILE_LIST))))

.PHONY: install
install:
	@npm ci

.PHONY: dev
dev: install
	@watchexec -N -c -rw $(ENTRYPOINT) --stop-signal SIGKILL node main.mjs

.PHONY: FORCE
FORCE:
