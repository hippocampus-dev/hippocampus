.DEFAULT_GOAL := all

TARGETS := target/x86_64-unknown-linux-gnu

.PHONY: fmt
fmt:
	@echo fmt
	@cargo fmt

.PHONY: lint
lint:
	@$(MAKE) -f ../hack/serial-makefile/Makefile lint

.PHONY: clean
clean/%:
	@echo $@
	@cargo clean -q --release --target $(@F)

.PHONY: build/l7sniff
build/l7sniff/%:
	@echo $@
	@cross build -q --bin l7sniff --release --timings --target $(@F)

.PHONY: build/insight
build/insight/%:
	@echo $@
	@cross build -q --bin insight --release --timings --target $(@F)

.PHONY: test
test/%:
	@echo $@
	@cross test -q --lib --release --target $(@F)
	@cross test -q --bin l7sniff --release --target $(@F)
	@cross test -q --bin insight --release --target $(@F)

target/%: FORCE clean/% build/l7sniff/% build/insight/% test/%
	@

targets: $(TARGETS)
	@

.PHONY: all
all: fmt lint $(TARGETS)
	@

.PHONY: dev
dev:
	@$(MAKE) -f ../hack/serial-makefile/Makefile dev

.PHONY: FORCE
FORCE:
