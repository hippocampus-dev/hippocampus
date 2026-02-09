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

### Language-Specific Commands

#### Rust Projects
- `cargo fmt` - Format code
- `cargo clippy --fix` - Lint and fix issues
- `cargo test` - Run tests
- `cargo test <test_name>` - Run a specific test
- `cargo test --package <package_name>` - Run tests for a specific package
- `cargo udeps --all-targets --all-features` - Check for unused dependencies
- `cross build --target <target>` - Cross-platform builds
- `mold --run cargo build` - Fast builds with mold linker
- `cargo build --release` - Build optimized binaries

#### Python Projects
- `uv sync --frozen` - Install dependencies
- `uv run -- python -m unittest discover` - Run tests
- `uv run -- python -m unittest <module>.<TestClass>.<test_method>` - Run a specific test
- `uv lock` - Update dependency locks
- `make uv` - Update all UV lock files
- `uv pip install <package>` - Add new dependencies

#### Go Projects
- `go mod tidy` - Tidy dependencies
- `go test ./...` - Run all tests
- `go test -run <TestName>` - Run a specific test
- `go build` - Build binaries
- `make gomod` - Update all Go modules
- `go mod download` - Download dependencies

#### Tauri/Android Projects
- `make android-init` - Initialize Android project (configures Kotlin 2.1.0+ for tauri-plugin-gemini)
- `make android-dev` - Build and run on device/emulator in development mode
- `make android-build` - Build release APK
- `make android-log` - Stream filtered logcat output

### Kubernetes Development
- `skaffold dev --port-forward` - Deploy to local Kubernetes with hot reload
- `make docker-build` - Build Docker images
- `kubectl apply -f <manifest.yaml>` - Apply Kubernetes manifests
- `kubectl logs -f <pod-name>` - Follow pod logs
- `kind create cluster` - Create local Kubernetes cluster for testing

## High-Level Architecture

### Project Structure
- **`/packages/`** - Rust workspace with core libraries:
  - `hippocampus-core` - Core search/indexing functionality
  - `hippocampus-server` - HTTP server implementation
  - `hippocampus-standalone` - Standalone application
  - Various utility packages (JWT, retry, telemetry)

- **`/cluster/applications/`** - Kubernetes microservices:
  - AI/ML services (embedding-gateway, embedding-retrieval, memory-bank, whisper-worker, translator)
  - Infrastructure (various pod hooks, controllers, proxies)
  - Developer tools (bakery, jupyterhub, github-actions-runner)
  - Integrations (slack bot, API gateway)

- **`/armyknife/`** - CLI tool for various utilities (Go-based)
- **`/insight/`** - eBPF-based system monitoring tool (Rust-based)
- **`/taurim/`** - Tauri-based Android timer application (Rust + TypeScript)
- **`/terraform/`** - Infrastructure as Code for Google Cloud and Cloudflare resources

### Key Design Patterns
1. **Container-first**: Every application has a Dockerfile
2. **Cross-compilation**: All Rust binaries support multiple Linux targets
3. **Kubernetes-native**: Extensive use of controllers, webhooks, and operators
4. **Mixed package managers**: UV for Python, Cargo for Rust, Go modules
5. **Watchexec integration**: Most services support hot-reload during development

### Development Workflow
1. Use `make dev` in the root or service directories for auto-rebuild
2. Run `make lint` and `make test` before committing
3. Use `cross` for building Linux binaries on any platform
4. Python projects use UV (not pip) for dependency management
5. Each service typically has its own Makefile with service-specific commands

## Important Notes
- The project uses `mold` linker for faster Rust builds when available
- Cross-compilation targets both GNU and musl libc for maximum compatibility
- Many services integrate with Kubernetes APIs and require cluster access
- Python projects are transitioning from Poetry to UV for dependency management

## Additional Development Commands & Patterns

### CI/CD and GitHub Actions
- **Dynamic Runner Selection**: The project uses a reusable workflow (`reusable_dynamic-runner.yaml`) that intelligently selects between self-hosted and GitHub-hosted runners
- **Security Scanning**: Docker image builds include vulnerability scanning via Trivy (`reusable_scan-image.yaml`). Application builds add scan jobs individually, while mirrored images are scanned via `99_scan-mirrored-images.yaml` (triggered by workflow_run, CRITICAL severity, with automatic per-CVE issue creation). Created issues require triage verification of actual exploitability including source code review and CVSS Attack Vector assessment
- **Claude AI Integration**: GitHub Actions support Claude AI interactions via `@claude` mentions in issues and PRs
- **Automated Dependency Updates**: `make poetry` (Poetry lockfile updates), `make uv` (UV lockfile updates), `make gomod` (Go module updates)

### Container Development
- `docker-compose up --profile=<profile>` - Run specific AI/ML stacks:
  - `stable-diffusion-webui` - Stable Diffusion WebUI
  - `stable-diffusion-webui-forge` - Stable Diffusion WebUI Forge
  - `comfyui` - ComfyUI
  - `llama.cpp` - LLaMA.cpp server
  - `yue` - Yue server
  - `open-webui` - Open WebUI for LLMs
- Docker Compose includes extensive observability stack (Prometheus, Grafana, Jaeger, Pyroscope)
- Registry mirrors for Docker Hub and GitHub Container Registry are configured

### Testing Patterns
- **K6 Load Testing**: Performance tests for various services (e.g., `cluster/applications/tcp-proxy/k6/connections.js`)
- **Parallel Test Scripts**: Some services include custom test scripts (e.g., `test_server_parallel.sh` for rust_de_llama)
- **Controller Testing**: Kubernetes controllers often include test environments using Kind (e.g., `make dev` in github-actions-runner-controller)

### Security and Encryption
- `make watch-decrypt` - Watch for encrypted files (*.enc) and decrypt them automatically
- `bin/decrypt.sh` - Decrypt all encrypted files in the project
- Uses `armyknife rails credentials:show` for decryption

### Kubernetes-Specific Patterns
- **CRD Generation**: Controllers use `controller-gen` for generating CRDs (e.g., github-actions-runner-controller)
- **Skaffold Integration**: Many Kubernetes apps support `skaffold dev` for hot-reload development
- **Kind Integration**: Some services include Kind configuration for local Kubernetes testing

### JavaScript/Frontend Projects
- **Preact Usage**: Frontend components use the `h` function from Preact (not JSX)
- Located in various application directories (e.g., `csviewer`, `kube-crud`, `talk`)
- No centralized package.json - each frontend is self-contained

### Utility Scripts
- `setup.sh` - System setup script that creates symlinks, configures development environment
- `bin/github-local-self-hosted-runner.sh` - Set up GitHub Actions self-hosted runners
- `bin/repository-settings.sh` - Configure repository settings
- Various maintenance scripts in `/bin` directory

### Observability
- Built-in support for OpenTelemetry, Jaeger tracing, Prometheus metrics
- Grafana dashboards pre-configured for system and application monitoring
- Pyroscope for continuous profiling

### MCP (Model Context Protocol) Servers
- Multiple MCP servers configured: GitHub, Playwright, filesystem, inspector
- Used for AI model integrations and browser automation

## Terraform Directory

### Structure
- `main.tf` - Module calls for Google, Cloudflare, and GitHub
- `outputs.tf` - Root module outputs referencing child module outputs
- `versions.tf` - Terraform and provider version constraints
- `providers.tf` - Provider configurations (Google, Cloudflare, GitHub)
- `variables.tf` - Input variable definitions
- `terraform.tfvars.example` - Example variable values
- `google/` - Google Cloud module (Storage buckets, Workload Identity Federation, service accounts)
- `cloudflare/` - Cloudflare module (Zone, Pages, KV, R2, Email Routing, DNS Records, Web Analytics, Notification Policies, DNSSEC, Tiered Cache, Bot Management)
- `github/` - GitHub module (repository rulesets)

### CI/CD
- GitHub Actions workflow (`50_terraform.yaml`) runs `terraform plan` on PRs
- Uses Workload Identity Federation for keyless GCP authentication

### Common Terraform Commands
- `terraform init` - Initialize Terraform working directory
- `terraform plan` - Preview infrastructure changes
- `terraform apply` - Apply infrastructure changes
- `terraform fmt` - Format Terraform files
- `terraform validate` - Validate configuration syntax

## Development Best Practices

### Code Consistency (CRITICAL)
Before writing any code, ALWAYS find and examine 3+ existing files with similar functionality:

1. **Search for similar files**: Use grep/glob to find files with the same language, framework, or purpose
2. **Examine patterns**: Identify structure, naming, imports, error handling, feature flags, dependencies
3. **Copy exactly**: Use existing patterns verbatim - do not invent new approaches
4. **Verify match**: Diff new code against reference to ensure consistency

| What to Check | Examples |
|---------------|----------|
| Dependency declaration | Optional vs non-optional, version format, feature flags |
| Feature flag pattern | `cfg!()` vs `#[cfg()]`, empty feature vs feature with deps |
| Error handling | Custom error types, Result aliases, error propagation |
| Initialization | Runtime setup, tracing, shutdown patterns |
| Import style | Full paths vs use statements, grouping, ordering |

Do NOT:
- Assume a pattern based on general knowledge
- Mix patterns from different projects
- Create "improvements" without explicit request

### Kubernetes Manifest Workflow
1. Find and copy existing similar manifests
2. Maintain existing structure while modifying for requirements
3. Update `kustomization.yaml` if exists, or create new one
4. For new `kustomization.yaml`, create corresponding file in `cluster/manifests/argocd-applications/base/`
5. When moving or adding internal image references, update `env.KUSTOMIZATION` in the corresponding `.github/workflows/00_*.yaml` workflow

### Common Gotchas
- Always check if a library is already used in the project before adding new dependencies
- Follow existing code conventions in neighboring files
- Never commit secrets or API keys
- Use `make lint` and `make test` before committing changes
- For encrypted files, use `make watch-decrypt` or `bin/decrypt.sh`
