# Technology Stack

## Build System
- **Make**: Primary build orchestration with language-specific targets
- **Cargo Workspaces**: Rust monorepo management
- **Cross**: Cross-platform compilation for Linux targets (x86_64-gnu, x86_64-musl)
- **Mold**: Fast linker for Rust builds when available
- **Watchexec**: Auto-rebuild during development

## Languages & Frameworks

### Rust
- **Core Libraries**: hippocampus-core, hippocampus-server, hippocampus-standalone
- **Async Runtime**: Tokio
- **Web Frameworks**: Axum, Warp
- **Serialization**: Serde, Bincode, Postcard
- **Database**: SQLite, Redis clients
- **Testing**: Built-in test framework, cargo-nextest

### Go
- **CLI Tools**: armyknife multi-tool, system utilities
- **Kubernetes**: Controllers, operators, webhooks
- **Web**: Echo, Gin frameworks
- **gRPC**: Protocol buffers, gRPC servers/clients

### Python
- **Package Manager**: UV (replacing Poetry)
- **Web Frameworks**: FastAPI, Django
- **AI/ML**: OpenAI SDK, Transformers, NumPy, Pandas
- **Async**: asyncio, aiohttp

### JavaScript/TypeScript
- **Frontend**: Preact (using h function, no JSX)
- **Build Tools**: esbuild, Vite
- **Runtime**: Node.js, Deno

## Infrastructure

### Kubernetes
- **Deployment**: Kustomize, Helm charts
- **Development**: Skaffold for hot-reload
- **Testing**: Kind for local clusters
- **Operators**: Kubebuilder patterns

### Containers
- **Docker**: Multi-stage builds, distroless images
- **Docker Compose**: Local development stacks with profiles
- **Registry**: GitHub Container Registry, local registry mirrors

### Cloud & IaC
- **Terraform**: Google Cloud, Cloudflare providers
- **Storage**: Google Cloud Storage, Cloudflare R2

## Observability
- **Metrics**: Prometheus, OpenTelemetry
- **Tracing**: Jaeger, OpenTelemetry
- **Logging**: Fluentd, Loki
- **Visualization**: Grafana
- **Profiling**: Pyroscope

## Common Commands

### Global Development
```bash
make dev                # Watch files and auto-rebuild
make all                # Format, lint, test, build, update deps
make fmt                # Format all code
make lint               # Lint and auto-fix issues
make test               # Run all tests
make targets            # Cross-compile for Linux targets
```

### Rust Development
```bash
cargo fmt               # Format Rust code
cargo clippy --fix      # Lint and fix Rust code
cargo test              # Run tests
cargo test <name>       # Run specific test
cargo udeps             # Check unused dependencies
mold --run cargo build  # Fast build with mold
```

### Python Development
```bash
uv sync --frozen        # Install dependencies
uv run pytest           # Run tests
uv lock                 # Update dependency locks
make uv                 # Update all UV lockfiles
```

### Go Development
```bash
go mod tidy             # Tidy dependencies
go test ./...           # Run all tests
go test -run <name>     # Run specific test
make gomod              # Update all Go modules
```

### Kubernetes Development
```bash
skaffold dev --port-forward     # Deploy with hot-reload
kubectl apply -f <manifest>     # Apply manifests
make -C <service> dev           # Service-specific dev mode
kind create cluster             # Create local test cluster
```

### Container Development
```bash
docker-compose up --profile=<profile>   # Run specific stack
make docker-build                       # Build all images
docker buildx build --platform linux/amd64,linux/arm64  # Multi-arch build
```
