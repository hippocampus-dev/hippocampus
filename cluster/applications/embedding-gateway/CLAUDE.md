# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

The embedding-gateway is a FastAPI-based microservice that acts as a caching proxy for OpenAI embeddings API requests. It uses content-addressable storage in S3/MinIO to cache embedding responses, reducing API costs and improving performance.

## Common Development Commands

### Development
- `make dev` - Run the service with hot-reload (watchexec) - main development command
- `make install` - Install dependencies with UV

### Testing & Quality
- `uv run -- python -m pytest` - Run tests
- `uv run -- python -m ruff check .` - Lint code
- `uv run -- python -m ruff format .` - Format code
- `uv run -- python -m mypy .` - Type checking (if configured)

### Dependencies
- `uv lock` - Update dependency lock file
- `uv sync --frozen` - Install exact dependencies from lock file
- `uv add <package>` - Add new dependency
- `uv remove <package>` - Remove dependency

### Docker & Kubernetes
- `docker build -t embedding-gateway .` - Build Docker image
- Deployment via ArgoCD using Kustomize manifests in `/opt/hippocampus/cluster/manifests/embedding-gateway/`

## High-Level Architecture

### Core Components

1. **Caching Proxy Flow** (`main.py:embeddings_endpoint`):
   - Receives POST request to `/openai/v1/embeddings`
   - Generates SHA-256 hash of request body as cache key
   - Checks S3 bucket for cached response at `embeddings/{hash}.json.gz`
   - Cache hit: Returns decompressed cached embeddings
   - Cache miss: Calls OpenAI API, compresses response, stores in S3, returns to client
   - All operations are async for high concurrency

2. **Observability Stack**:
   - **OpenTelemetry** (`telemetry.py`): Full instrumentation with traces and metrics
   - **Structured Logging** (`context_logging.py`): JSON logs with trace context injection
   - **Prometheus Metrics**: Exposed at `/metrics` endpoint
   - **Health Check**: Available at `/healthz` endpoint
   - Key metrics tracked: request latency, cache hit rate, OpenAI API errors

3. **Configuration Management** (`settings.py`):
   - Environment-based configuration using Pydantic Settings
   - Critical settings:
     - `S3_BUCKET`: Cache storage bucket (default: `embedding-gateway`)
     - `S3_ENDPOINT_URL`: S3/MinIO endpoint for local dev
     - `OPENAI_BASE_URL`: Override OpenAI API base URL
     - `HOST`/`PORT`: Server binding configuration
     - `LOG_LEVEL`: Logging verbosity
     - `OTEL_*`: OpenTelemetry configuration

### Key Design Patterns

1. **Content-Addressable Caching**:
   - SHA-256 hash of entire request body ensures deterministic cache keys
   - Identical requests always result in cache hits
   - Cache stored as gzip-compressed JSON

2. **Error Handling** (`exceptions.py`):
   - `EmbeddingGatewayException`: Base exception class
   - `CacheError`: S3 operation failures
   - `OpenAIError`: Upstream API failures
   - Distinguishes retryable (503) vs non-retryable errors

3. **Async/Await Architecture**:
   - FastAPI with uvicorn for async request handling
   - All I/O operations (S3, OpenAI) are async
   - Enables handling hundreds of concurrent requests

4. **Security Hardening**:
   - Runs as non-root user (UID 65532) in production
   - Read-only root filesystem in Kubernetes
   - No secrets in code - all via environment variables

### Kubernetes Integration

The service is deployed using Kustomize with the following structure:
- **Base manifests** (`/cluster/manifests/embedding-gateway/base/`):
  - `deployment.yaml`: Core deployment with security contexts
  - `service.yaml`: ClusterIP service for internal access
  - `horizontal_pod_autoscaler.yaml`: Auto-scaling based on CPU/memory
  - `pod_disruption_budget.yaml`: Ensures availability during updates

- **Environment overlays** (`/cluster/manifests/embedding-gateway/overlays/`):
  - `dev/`: Development-specific patches

- **Istio Integration**:
  - Service mesh for mTLS, traffic management
  - VirtualService/DestinationRule for routing
  - PeerAuthentication for security

### Important Implementation Details

1. **Cache Storage Format**:
   - Path: `embeddings/{sha256_hash}.json.gz`
   - Content: Gzip-compressed JSON matching OpenAI API response format
   - Compression typically reduces size by 60-80%

2. **Request Flow**:
   ```
   Client → Istio Gateway → embedding-gateway Service → Pod
   ↓
   Check S3 Cache (async)
   ↓ (miss)
   Call OpenAI API (async)
   ↓
   Compress & Store in S3 (async)
   ↓
   Return response to client
   ```

3. **Performance Considerations**:
   - S3 operations use connection pooling
   - OpenAI client reuses HTTP connections
   - All blocking I/O is avoided in request path

4. **Monitoring Integration**:
   - Traces sent to Jaeger/Tempo
   - Metrics scraped by Prometheus
   - Logs collected by Loki/Fluentd
   - Dashboards in Grafana

## Development Workflow

1. Start development server: `make dev`
2. Service runs on `http://localhost:8000` by default
3. Test endpoint: `curl http://localhost:8000/healthz`
4. Make changes - server auto-reloads via watchexec
5. Format code: `uv run -- python -m ruff format .`
6. Check linting: `uv run -- python -m ruff check .`
7. Run tests: `uv run -- python -m pytest`
8. Update dependencies: Edit `pyproject.toml` then run `uv lock`

## Testing Approach

While no test files currently exist, tests should be added to:
- Unit test cache key generation
- Mock S3 operations for cache testing
- Mock OpenAI API for error handling
- Integration tests with real S3/MinIO
- Load tests to verify async performance

Test files should be placed in `tests/` directory and follow pytest conventions.
