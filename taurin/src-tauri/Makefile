.DEFAULT_GOAL := all

.PHONY: fmt
fmt:
	@echo fmt
	@cargo fmt

.PHONY: lint
lint:
	@$(MAKE) -f ../../hack/serial-makefile/Makefile lint

.PHONY: all
all: fmt lint
	@

.PHONY: FORCE
FORCE:
