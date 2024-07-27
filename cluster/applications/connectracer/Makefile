.DEFAULT_GOAL := all

TARGETS := target/x86_64-unknown-linux-gnu

.PHONY: fmt
fmt:
	@echo fmt
	@cargo fmt

.PHONY: lint
lint:
	@$(MAKE) -f ../../../hack/serial-makefile/Makefile lint

.PHONY: clean
clean/%:
	@echo $@
	@cargo clean -q --release --target $(@F)

.PHONY: build
build/%:
	@echo $@
	@cross build -q --release --timings --target $(@F)

.PHONY: test
test/%:
	@echo $@
	@cross test -q --lib --release --target $(@F)

target/%: FORCE clean/% build/% test/%
	@

targets: $(TARGETS)
	@

.PHONY: all
all: fmt lint $(TARGETS)
	@

.PHONY: dev
dev:
	@$(MAKE) -f ../../../hack/serial-makefile/Makefile dev

.PHONY: FORCE
FORCE:
