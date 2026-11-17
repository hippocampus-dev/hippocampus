# Rust Makefile Pattern

## Template

```makefile
.DEFAULT_GOAL := all

ENTRYPOINT := $(dir $(abspath $(firstword $(MAKEFILE_LIST))))

TARGETS := target/x86_64-unknown-linux-gnu

.PHONY: fmt
fmt:
	@cargo fmt

.PHONY: lint
lint:
	@cargo clippy --fix --allow-dirty

.PHONY: tidy
tidy:
	@cargo udeps --all-targets --all-features

.PHONY: build
build/%:
	@cross build -q --release --timings --target $(@F)

.PHONY: test
test/%:
	@cross test -q --release --target $(@F)

target/%: FORCE build/% test/%
	@

targets: $(TARGETS)
	@

.PHONY: all
all: fmt lint tidy $(TARGETS)
	@

.PHONY: dev
dev:
	@watchexec -N -c -rw $(ENTRYPOINT) --stop-signal SIGKILL mold --run cargo build

.PHONY: FORCE
FORCE:
```

## Key Points

* Uses `cargo fmt` for formatting
* Uses `cargo clippy --fix` or `cargo udeps` for linting
* `cross` for cross-compilation to Linux targets
* `mold` linker for faster development builds
* `watchexec` for auto-rebuild during development
* Pattern rules (`%`) for multi-target builds
* `FORCE` target to ensure rebuilds
