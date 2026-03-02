# Node.js Makefile Pattern

## Pattern Selection

| Condition | Pattern | Example |
|-----------|---------|---------|
| Standalone script (no build step) | Standalone | `cluster/applications/libsodium-encryptor/` |
| Framework app (build required) | Framework | `docker-compose/comfyui/autocomfy/` |

## Standalone Template

For applications that run directly with `node`:

```makefile
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
```

## Framework Template

For applications with a build step (TanStack Start, Next.js, etc.):

```makefile
.DEFAULT_GOAL := all

.PHONY: install
install:
	@npm ci

.PHONY: build
build:
	@npm run build

.PHONY: all
all: install build
	@

.PHONY: dev
dev: install
	@npm run dev

.PHONY: FORCE
FORCE:
```

## Key Points

| Practice | Pattern |
|----------|---------|
| Install dependencies | `npm ci` (not `npm install`) for reproducible installs |
| Dev depends on install | `dev: install` ensures dependencies are present |
| Standalone dev | `watchexec` for auto-reload |
| Framework dev | `npm run dev` (framework provides hot reload) |
| No fmt/lint targets | Handled by framework tooling or not applicable |
