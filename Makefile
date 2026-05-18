.DEFAULT_GOAL := all

TARGETS := target/x86_64-unknown-linux-gnu target/x86_64-unknown-linux-musl
POETRY := $(shell find . -type f -name 'poetry.lock')
UV := $(shell find . -type f -name 'uv.lock')
GOMOD := $(shell find . -type f -name 'go.mod')

.PHONY: fmt
fmt:
	@cargo fmt

.PHONY: lint
lint:
	@$(MAKE) -f hack/serial-makefile/Makefile lint

.PHONY: tidy
tidy:
	@cargo udeps --all-targets --all-features

.PHONY: test
test:
	@cargo test

.PHONY: docker-build
docker-build:
	@docker build --build-arg bin=hippocampus-standalone -t hippocampus-standalone .

.PHONY: clean
clean/%:
	@echo $@
	@cargo clean -q --release --target $(@F)
	@rm target/debug/.fingerprint/tracing-*

.PHONY: build
build/%:
	@echo $@
	@cross build -q --bin hippocampus-standalone --release --timings --target $(@F)

.PHONY: test
test/%:
	@echo $@
	@cross test -q --lib --release --target $(@F)
	@cross test -q --bin hippocampus-standalone --release --target $(@F)

target/%: FORCE clean/% build/% test/%
	@

.PHONY: targets
targets: $(TARGETS)
	@

$(POETRY): FORCE
	@echo $@
	@(cd $$(dirname $@) && uvx poetry lock)

.PHONY: poetry
poetry: $(POETRY)
	@

$(UV): FORCE
	@echo $@
	@(cd $$(dirname $@) && uv lock --frozen)

.PHONY: uv
uv: $(UV)
	@

$(GOMOD): FORCE
	@echo $@
	@(cd $$(dirname $@) && go mod tidy)

.PHONY: gomod
gomod: $(GOMOD)
	@

.PHONY: all
all: fmt lint tidy $(TARGETS) $(POETRY) $(UV) $(GOMOD)
	@

.PHONY: dev
dev:
	@command -v update-ca-certificate > /dev/null && sudo cp -f ./.mitmproxy/mitmproxy-ca-cert.pem /usr/share/ca-certificates/mitmproxy-ca-cert.crt && sudo update-ca-certificates || true
	@command -v update-ca-trust > /dev/null && sudo cp -f ./.mitmproxy/mitmproxy-ca-cert.pem /etc/ca-certificates/trust-source/anchors/mitmproxy-ca-cert.crt && sudo update-ca-trust || true
	@command -v trust > /dev/null && sudo trust anchor --store ./.mitmproxy/mitmproxy-ca-cert.pem || true
	@$(MAKE) -f hack/serial-makefile/Makefile dev

.PHONY: watch-decrypt
watch-decrypt:
	@$(MAKE) -f hack/serial-makefile/Makefile watch-decrypt

.PHONY: FORCE
FORCE:
