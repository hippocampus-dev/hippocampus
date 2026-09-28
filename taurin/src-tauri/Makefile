.DEFAULT_GOAL := all

.PHONY: fmt
fmt:
	@cargo fmt

.PHONY: lint
lint:
	@$(MAKE) -f ../../hack/serial-makefile/Makefile lint

.PHONY: tidy
tidy:
	@cargo udeps --all-targets --all-features

.PHONY: test
test:
	@cargo test

.PHONY: all
all: fmt lint tidy test
	@

.PHONY: FORCE
FORCE:
