.DEFAULT_GOAL := all

TARGETS := target/x86_64-unknown-linux-gnu target/x86_64-unknown-linux-musl
PYPROJECTS := $(shell find . -type f -name "pyproject.toml")

.PHONY: fmt
fmt:
	@echo fmt
	@cargo fmt

.PHONY: lint
lint:
	@$(MAKE) -f hack/serial-makefile/Makefile lint

.PHONY: test
test:
	@echo test
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

targets: $(TARGETS)
	@

$(PYPROJECTS): FORCE
	@echo $@
	@(cd $$(dirname $@) && poetry lock --no-update)

projects: $(PYPROJECTS)
	@


.PHONY: all
all: fmt lint test $(TARGETS) $(PYPROJECTS)
	@

.PHONY: dev
dev:
	@docker compose up --build -d --wait
	@command -v update-ca-certificate > /dev/null && sudo cp -f ./.mitmproxy/mitmproxy-ca-cert.pem /usr/share/ca-certificates/mitmproxy-ca-cert.crt && sudo update-ca-certificates || true
	@command -v update-ca-trust > /dev/null && sudo cp -f ./.mitmproxy/mitmproxy-ca-cert.pem /etc/ca-certificates/trust-source/anchors/mitmproxy-ca-cert.crt && sudo update-ca-trust || true
	@command -v trust > /dev/null && sudo trust anchor --store ./.mitmproxy/mitmproxy-ca-cert.pem || true
	@trap 'docker compose down' INT; $(MAKE) -f hack/serial-makefile/Makefile dev

.PHONY: FORCE
FORCE:
