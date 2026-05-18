.DEFAULT_GOAL := all

TARGETS := target/x86_64-unknown-linux-gnu

.PHONY: fmt
fmt:
	@cargo fmt

.PHONY: lint
lint:
	@$(MAKE) -f ../../../hack/serial-makefile/Makefile lint

.PHONY: tidy
tidy:
	@cargo udeps --all-targets --all-features

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
	@cross test -q --release --target $(@F)

target/%: FORCE clean/% build/% test/%
	@

targets: $(TARGETS)
	@

.PHONY: all
all: fmt lint tidy $(TARGETS)
	@

.PHONY: dev
dev:
	@skaffold dev --port-forward

.PHONY: e2e
e2e:
	@kind create cluster --name proxy-wasm
	@trap 'kind delete cluster --name proxy-wasm' EXIT ERR INT; ./e2e.sh

.PHONY: FORCE
FORCE:
