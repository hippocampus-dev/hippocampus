# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Hippocampus is a full-stack Kubernetes platform that combines cloud-native technologies, AI/ML services, and developer tools. It's a monorepo supporting multiple programming languages with a container-first approach.

## Common Development Commands

### Global Commands
- `make dev` - Main development command that sets up CA certificates and runs watchexec for auto-building
- `make all` - Runs formatting, linting, testing, builds all targets, and updates dependencies
- `make fmt` - Formats all code (Rust with cargo fmt)
- `make lint` - Lints and auto-fixes code (Rust with cargo clippy --fix)
- `make test` - Runs all tests
- `make targets` - Cross-compiles for x86_64-unknown-linux-gnu and x86_64-unknown-linux-musl
- `make docker-build` - Build Docker images
- `make watch-decrypt` - Watch and auto-decrypt encrypted files
- `make uv` - Update all UV lock files in Python projects
- `make gomod` - Update all Go modules

### Language-Specific Commands

#### Rust Projects
- `cargo fmt` - Format code
- `cargo clippy --fix` - Lint and fix issues
- `cargo test` - Run tests
- `cargo test <test_name>` - Run a single test
- `cargo udeps --all-targets --all-features` - Check for unused dependencies
- `cross build --target <target>` - Cross-platform builds
- `mold --run cargo build` - Fast builds with mold linker

#### Python Projects
- `uv sync --frozen` - Install dependencies
- `uv run -- python -m unittest discover` - Run tests
- `uv run -- python -m pytest tests/<test_file.py>::<test_name>` - Run a single test
- `uv lock` - Update dependency locks

#### Go Projects
- `go mod tidy` - Tidy dependencies
- `go test ./...` - Run all tests
- `go test -run TestName` - Run a single test
- `go build` - Build binaries

#### TypeScript/JavaScript Projects
- Uses Preact with `h` function for element creation
- Frontend applications in `/cluster/applications/*/frontend/`

### Kubernetes Development
- `skaffold dev --port-forward` - Deploy to local Kubernetes with hot reload
- `skaffold run` - Deploy to Kubernetes without watching
- `kind create cluster` - Create local Kubernetes cluster
- `make generate` - Generate CRDs and manifests (in controller projects)
- `controller-gen crd` - Generate Custom Resource Definitions

### Testing Commands
- `make test-parallel` - Run tests in parallel (in some services)
- `k6 run test.js` - Run load tests
- `make test-controller` - Test Kubernetes controllers with Kind

### Container Development
- `docker compose --profile ai-ml up` - Start AI/ML stack
- `docker compose --profile observability up` - Start observability stack (Prometheus, Grafana, Jaeger, Pyroscope)
- Registry mirrors configured at `/etc/docker/daemon.json`

## High-Level Architecture

### Project Structure
- **`/packages/`** - Rust workspace with core libraries:
  - `hippocampus-core` - Core search/indexing functionality
  - `hippocampus-server` - HTTP server implementation
  - `hippocampus-standalone` - Standalone application
  - Various utility packages (JWT, retry, telemetry)

- **`/cluster/applications/`** - Kubernetes microservices:
  - AI/ML services (embedding-gateway, embedding-retrieval, whisper-worker, translator)
  - Infrastructure (various pod hooks, controllers, proxies)
  - Developer tools (bakery, jupyterhub, github-actions-runner)
  - Integrations (slack bot, API gateway)

- **`/armyknife/`** - CLI tool for various utilities
- **`/insight/`** - eBPF-based system monitoring tool

### Key Design Patterns
1. **Container-first**: Every application has a Dockerfile
2. **Cross-compilation**: All Rust binaries support multiple Linux targets
3. **Kubernetes-native**: Extensive use of controllers, webhooks, and operators
4. **Mixed package managers**: UV for Python, Cargo for Rust, Go modules
5. **Watchexec integration**: Most services support hot-reload during development
6. **Multi-stage Docker builds**: Optimized for size and security
7. **Distroless images**: Final images use gcr.io/distroless base

### Development Workflow
1. Use `make dev` in the root or service directories for auto-rebuild
2. Run `make lint` and `make test` before committing
3. Use `cross` for building Linux binaries on any platform
4. Python projects use UV (not pip) for dependency management
5. Each service typically has its own Makefile with service-specific commands

### CI/CD and GitHub Actions
- Dynamic runner selection based on labels
- Claude AI integration via `@claude` mentions in PRs
- Automatic dependency updates with dedicated workflows
- Self-hosted runners managed by `github-actions-runner-controller`

### Security and Encryption
- Encrypted files auto-decrypted with `make watch-decrypt`
- libsodium-based encryption for sensitive data
- JWT authentication across services

### Observability
- OpenTelemetry integration across all services
- Jaeger for distributed tracing
- Prometheus and Grafana for metrics
- Pyroscope for continuous profiling
- Pre-configured Grafana dashboards in `/cluster/applications/grafana/`

## Important Notes
- The project uses `mold` linker for faster Rust builds when available
- Cross-compilation targets both GNU and musl libc for maximum compatibility
- Many services integrate with Kubernetes APIs and require cluster access
- Python projects are transitioning from Poetry to UV for dependency management
- All services support structured logging and OpenTelemetry tracing
- MCP servers configured for GitHub, filesystem, and browser automation