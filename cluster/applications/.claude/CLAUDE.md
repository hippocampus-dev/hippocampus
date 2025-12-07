# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This directory (`cluster/applications/`) contains 50+ microservices that form a polyglot Kubernetes platform. Each application is a self-contained service with its own Dockerfile, Makefile, and language-specific dependencies. The codebase follows strong standardization patterns across Go, Python, Rust, and JavaScript applications.

## Common Development Commands

All applications follow a consistent Makefile pattern. Navigate to any application directory and use:

### Universal Commands (All Languages)
- `make dev` - Start development with auto-reload (watchexec-based)
- `make all` - Run full test suite: format, lint, tidy, test
- `make fmt` - Format code according to language standards
- `make lint` - Lint and auto-fix code issues
- `make test` - Run tests (with race detection for Go)

### Go Applications (20 services)
```bash
make fmt      # go fmt + goimports
make lint     # go vet
make tidy     # go mod tidy
make test     # go test -race ./...
go test -run <TestName>  # Run specific test
```

### Python Applications (13 services)
```bash
make install  # uv sync --frozen (+ playwright install if needed)
make fmt      # uvx ruff format .
make lint     # uvx ruff check --fix .
uv run -- python -m unittest discover  # Run all tests
uv run -- python -m unittest <module>.<TestClass>.<test_method>  # Run specific test
```

### Rust Applications (4 services)
```bash
make targets  # Cross-compile for x86_64-unknown-linux-gnu and x86_64-unknown-linux-musl
make dev      # watchexec with mold --run cargo build
cargo test    # Run tests
cross build --target x86_64-unknown-linux-musl  # Cross-compile
```

### Kubernetes Development
```bash
skaffold dev --port-forward  # Deploy to local Kubernetes with hot reload (if skaffold.yaml exists)
kubectl apply -k manifests/  # Apply production manifests
kubectl apply -k skaffold/   # Apply development manifests
```

## High-Level Architecture

### Directory Structure Pattern

Each application follows a standard layout:
```
<application-name>/
├── Dockerfile           # Multi-stage build (builder + runtime)
├── Makefile            # Standard targets: dev, all, fmt, lint, test
├── manifests/          # Production Kubernetes manifests
│   ├── kustomization.yaml
│   ├── deployment.yaml
│   ├── service.yaml
│   └── ...
├── skaffold/           # Development Kubernetes manifests
│   ├── kustomization.yaml
│   └── patches/
├── skaffold.yaml       # Skaffold configuration (if applicable)
└── AGENTS.md           # Application-specific Claude Code guidance
```

Language-specific files:
- **Go**: `go.mod`, `go.sum`, `main.go`
- **Python**: `pyproject.toml`, `uv.lock`, `main.py`
- **Rust**: `Cargo.toml`, `Cargo.lock`, `src/main.rs`
- **JavaScript**: `package.json`, `package-lock.json`

### Application Categories

1. **Kubernetes Controllers & Webhooks** (13 apps)
   - Pod lifecycle hooks: `at-least-semaphore-pod-hook`, `exactly-one-pod-hook`, `job-hook`, `lifecycle-job-hook`, `litestream-hook`, `persistentvolumeclaim-hook`
   - Controllers: `github-actions-runner-controller`, `nodeport-controller`, `snapshot-controller`, `owasp-zap-controller`
   - Event loggers: `events-logger`, `cloudevents-logger`

2. **AI/ML Services** (7 apps)
   - `api` - OpenAI-compatible chat API with agent capabilities
   - `bot` - Enterprise Slack bot with multi-modal AI
   - `embedding-gateway`, `embedding-retrieval`, `embedding-retrieval-loader` - Semantic search
   - `whisper-worker` - Speech-to-text
   - `translator` - Translation services
   - `memory-bank` - AI memory storage

3. **Infrastructure Services** (10 apps)
   - Proxies: `anonymous-proxy`, `configurable-http-proxy`, `redis-proxy`, `tcp-proxy`, `mcp-stdio-proxy`
   - Logging: `slack-logger`, `fluentd-aggregator`, `fluentd-delayed-unlink`
   - Monitoring: `connectracer` (eBPF), `realtime-search-exporter`, `lighthouse-exporter`

4. **Developer Tools** (6 apps)
   - `jupyterhub` - Interactive Python notebooks
   - `bakery` - Build automation
   - `runner` - GitHub Actions runner
   - `k6` - Load testing
   - `playwright-mcp` - Browser automation
   - `jsonnet-builder` - Configuration builder

5. **Web Services** (5 apps)
   - `kube-crud`, `kube-crud-server` - Web UI for Kubernetes
   - `csviewer` - CSV viewing
   - `headless-page-renderer` - Server-side rendering
   - `talk` - Chat application

### Key Design Patterns

1. **Containerization**
   - Multi-stage Dockerfiles with builder and runtime stages
   - Non-root user (UID 65532) for security
   - Distroless base images for Go applications
   - Minimal slim-bookworm base for Python applications

2. **Kubernetes Manifests**
   - Dual manifest structure: `manifests/` (production) and `skaffold/` (development)
   - Kustomize for composition and patching
   - Standard security contexts (runAsNonRoot, readOnlyRootFilesystem, DROP ALL capabilities)
   - Health probes on all deployments
   - PodDisruptionBudgets for high-availability services

3. **Development Workflow**
   - `watchexec` for auto-reload during development
   - `skaffold dev` for local Kubernetes testing
   - Language-specific formatters and linters enforced via Makefiles
   - Frozen dependencies via lockfiles (uv.lock, go.sum, Cargo.lock)

4. **Observability**
   - OpenTelemetry integration in Python services
   - Prometheus metrics endpoints (`/metrics`)
   - Health check endpoints (`/healthz` or `/health`)
   - Structured logging with context

## Application-Specific Guidance

Each application has an `AGENTS.md` file (formatted as CLAUDE.md) that provides detailed, application-specific guidance. Always check the `AGENTS.md` file in the application directory before making changes.

Example applications with comprehensive AGENTS.md files:
- `api/AGENTS.md` - API service architecture and agent system
- `bot/AGENTS.md` - Slack bot agent-based architecture and load balancing
- `github-actions-runner-controller/AGENTS.md` - Controller patterns and CRD generation

## Development Best Practices

### Adding a New Application

1. Copy an existing application with similar language/framework as a template
2. Update the application name in all files (Dockerfile, Makefile, manifests)
3. Ensure Makefile has standard targets: `dev`, `all`, `fmt`, `lint`, `test`
4. Add multi-stage Dockerfile with builder and runtime stages
5. Create both `manifests/` and `skaffold/` directories if deploying to Kubernetes
6. Add `AGENTS.md` file with application-specific guidance

### Modifying Kubernetes Manifests

1. Check if application has both `manifests/` and `skaffold/` directories
2. Modify production manifests in `manifests/`
3. If using Kustomize, update `kustomization.yaml` to include new resources
4. For development overrides, add patches to `skaffold/patches/`
5. Test with `skaffold dev --port-forward` before committing

### Language-Specific Notes

**Go Applications:**
- Use Go 1.24+
- Enable race detection in tests: `go test -race ./...`
- CGO_ENABLED=0 for static binaries
- Use distroless base images in Dockerfile

**Python Applications:**
- Use UV package manager (not pip or poetry)
- Python 3.11 minimum
- Install with `uv sync --frozen` (never modify uv.lock manually)
- Use `uvx` for running tools (ruff, pytest) without installation

**Rust Applications:**
- Cross-compile for both x86_64-unknown-linux-gnu and x86_64-unknown-linux-musl
- Use `cross` tool for cross-compilation
- Use `mold` linker for faster builds during development
- eBPF applications (like connectracer) require special build tools

### Testing Before Committing

Always run before committing:
```bash
make all  # Runs fmt, lint, tidy, test in sequence
```

For Kubernetes changes:
```bash
skaffold dev --port-forward  # Test in local cluster
kubectl apply --dry-run=client -k manifests/  # Validate manifests
```

## Common Gotchas

- **UV vs pip**: Python projects use UV exclusively. Never use `pip install` - use `uv pip install` instead
- **Frozen dependencies**: Never modify lockfiles (uv.lock, go.sum, Cargo.lock) manually. Use package manager commands
- **Non-root user**: All containers run as UID 65532. Ensure file permissions are correct
- **Cross-compilation**: Rust applications must build for both gnu and musl targets
- **Makefile targets**: Always use `make dev` for development, not direct language commands
- **Skaffold vs manifests**: `skaffold/` is for development, `manifests/` is for production. Don't confuse them
- **AGENTS.md files**: Check application-specific AGENTS.md before making changes - they contain important context
