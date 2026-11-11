# Project Structure

## Root Directory Layout

```
/opt/hippocampus/
    packages/           # Rust workspace with core libraries
    cluster/           # Kubernetes applications and manifests
    armyknife/         # Go CLI multi-tool
    insight/           # eBPF system monitoring tool
    terraform/         # Infrastructure as Code
    docker-compose/    # Local development environments
    bin/              # Utility scripts
    .github/          # GitHub Actions workflows
    Makefile          # Root build orchestration
```

## Core Directories

### `/packages/` - Rust Workspace
Monorepo for Rust libraries and applications using Cargo workspaces:
- `hippocampus-core/` - Core search and indexing functionality
- `hippocampus-server/` - HTTP server implementation
- `hippocampus-standalone/` - Standalone application
- `jwt/` - JWT authentication utilities
- `retry/` - Retry logic utilities
- `opentelemetry-tracing/` - OpenTelemetry integration
- Additional utility crates for specific functionality

### `/cluster/` - Kubernetes Resources
- `applications/` - Individual microservices and applications
  - Each app has: `Dockerfile`, `Makefile`, source code, K8s manifests
  - Examples: `embedding-gateway/`, `whisper-worker/`, `slack-logger/`
- `manifests/` - Kubernetes manifests organized by component
  - `argocd-applications/` - ArgoCD app definitions
  - `cert-manager/`, `istio-gateways/`, `prometheus/` - Infrastructure components
- Secrets are stored in Vault (`cluster/bin/initialize-vault.sh`) and consumed via `SecretsFromVault` manifests

### `/armyknife/` - Go Multi-Tool
Comprehensive CLI utility with subcommands:
- `cmd/` - Command implementations
- `internal/` - Internal packages
- Subcommands for Rails, S3, gRPC, MCP, and more

### `/insight/` - System Monitoring
eBPF-based monitoring tool:
- `src/` - Rust source code and eBPF programs
- Network, CPU, and HTTP/HTTPS tracing capabilities

### `/terraform/` - Infrastructure as Code
- `main.tf` - Primary resource definitions
- `providers.tf` - Provider configurations
- `variables.tf` - Input variables
- `outputs.tf` - Output values
- `versions.tf` - Provider version constraints

## Service Structure Pattern

Most services follow this structure:
```
service-name/
    Dockerfile          # Container definition
    Makefile           # Service-specific commands
    src/               # Source code (language-specific)
    tests/             # Test files
    manifests/         # Kubernetes manifests (optional)
    README.md          # Service documentation
```

## Configuration Files

### Build & Development
- `Makefile` - Present at root and in most subdirectories
- `Cargo.toml` / `Cargo.lock` - Rust dependencies
- `go.mod` / `go.sum` - Go dependencies
- `pyproject.toml` / `uv.lock` - Python dependencies
- `package.json` - JavaScript dependencies (per-service)

### Kubernetes & Docker
- `kustomization.yaml` - Kustomize configurations
- `docker-compose.yaml` - Local development stacks
- `skaffold.yaml` - Kubernetes development workflow

### CI/CD
- `.github/workflows/` - GitHub Actions workflows
- Dynamic runner selection and Claude AI integration

## Language-Specific Patterns

### Rust Projects
- Workspace members defined in root `Cargo.toml`
- Cross-compilation targets in `Cross.toml`
- Binary outputs to `target/` directory

### Go Projects
- Module-based with `go.mod` at project root
- Internal packages in `internal/` directory
- Command entrypoints in `cmd/` directory

### Python Projects
- UV package manager with `pyproject.toml`
- Virtual environments managed by UV
- FastAPI/Django apps with standard structures

### JavaScript/TypeScript
- No central package.json - each service is independent
- Preact components using h function (no JSX)
- Build outputs typically in `dist/` or `build/`
