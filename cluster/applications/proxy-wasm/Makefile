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
	@skaffold dev --port-forward

.PHONY: e2e
e2e:
	@kind create cluster --name proxy-wasm
	@skaffold run
	@kubectl port-forward svc/skaffold-proxy-wasm 8080:8080 8081:8081 -n skaffold-proxy-wasm > /dev/null 2>&1 &
	@cat k6/fallback-filter/index.js | docker compose exec -T k6 k6 run -
	@cat k6/cookie-manipulator/request.js | docker compose exec -T k6 k6 run -
	@cat k6/cookie-manipulator/response.js | docker compose exec -T k6 k6 run -
	@kind delete cluster --name proxy-wasm

.PHONY: FORCE
FORCE:
