# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

slack-logger is a Go-based HTTP service that receives Slack events, encrypts message data, and stores it in a MySQL-compatible database (TiDB). It provides API endpoints to retrieve encrypted messages and integrates with Kubernetes event streaming via Knative and Kafka.

## Application Location

- **Source code**: `/opt/hippocampus/cluster/applications/slack-logger`
- **Kubernetes manifests**: `/opt/hippocampus/cluster/manifests/slack-logger`

## Common Development Commands

### Building and Running
- `make dev` - Run with auto-reload using watchexec
- `go run *.go` - Run the application directly
- `make all` - Format, lint, tidy, and test
- `make fmt` - Format code with `go fmt` and `goimports`
- `make lint` - Lint code with `go vet`
- `make tidy` - Tidy Go module dependencies
- `make test` - Run tests with race detection and benchmarks

### Database Schema Management
- `make validate` - Validate Atlas migrations and sqlc diff
- `make generate` - Generate new migration with Atlas and regenerate sqlc code
- `make migrate` - Apply migrations to remote database (requires environment variables)

The project uses:
- **Atlas** for database schema migrations (defined in `schema.hcl`)
- **sqlc** for type-safe SQL query code generation (queries in `queries/messages.sql`)

### Kubernetes Deployment
- `kubectl apply -k overlays/dev` - Deploy to dev environment
- `kubectl logs -f -n slack-logger deployment/slack-logger` - View application logs
- `kubectl get tidbcluster -n slack-logger` - Check TiDB cluster status

## High-Level Architecture

### Core Components

1. **HTTP Server** (`main.go`)
   - Listens on port 8080
   - Implements graceful shutdown with lameduck period
   - Full observability: OpenTelemetry traces, Prometheus metrics, Pyroscope profiling
   - Custom middleware for request handling with panic recovery
   - Connection limiting via `netutil.LimitListener`

2. **Event Handler** (`internal/routes/events.go`)
   - `/slack/events` - Receives Slack Event API callbacks
   - Handles URL verification challenges
   - Processes message events: new messages, edits (`message_changed`), deletions (`message_deleted`)
   - Encrypts message payloads before storage

3. **API Endpoints** (`internal/routes/conversations_*.go`)
   - `/api/conversations.history` - Retrieve channel messages with pagination
   - `/api/conversations.replies` - Retrieve thread messages with pagination
   - Support time-based filtering with inclusive/exclusive bounds

4. **Encryption** (`internal/encryption/encryption.go`)
   - Uses AES-256-GCM for authenticated encryption
   - PBKDF2 key derivation (600,000 iterations) with per-message salt
   - Channel ID serves as encryption key material
   - Salt is stored alongside encrypted data

5. **Storage Layer** (`internal/storage/mysql.go`, `internal/db/`)
   - MySQL/TiDB backend with sqlc-generated queries
   - OpenTelemetry instrumentation for database operations
   - Connection pooling with configurable limits

### Database Schema

Single table `encrypted_messages`:
- Primary key: auto-incrementing `id`
- Unique index on `(channel_name, message_ts)` for upserts
- Index on `(channel_name, timestamp)` for time-based queries
- Index on `(channel_name, thread_ts)` for thread retrieval
- Stores encrypted message data with salt

### Kubernetes Integration

The service deploys with:
- **TiDB Cluster**: MySQL-compatible distributed database (PD, TiKV, TiDB components)
- **TiCDC**: Change Data Capture streaming changes to Kafka
- **Kafka**: Event streaming via Strimzi operator
- **KafkaSource**: Knative eventing integration forwarding CDC events to `cloudevents-logger` broker
- **Atlas Operator**: Automated schema migrations via `AtlasSchema` CRD

### Observability Stack

- **Metrics**: Prometheus exposition at `/metrics` with OpenMetrics support
- **Tracing**: OpenTelemetry with OTLP gRPC exporter
- **Profiling**: Continuous profiling with Pyroscope (CPU, memory, goroutines, mutex, block)
- **Logging**: Structured JSON logs (slog) with OpenTelemetry semantic conventions
- **Health**: `/healthz` endpoint for Kubernetes probes

## Development Workflow

1. **Making Schema Changes**:
   - Edit `schema.hcl` with desired changes
   - Run `make generate` to create migration and regenerate Go code
   - Review generated migration in `migrations/`
   - Test locally with `atlas migrate validate`
   - Deploy with `make migrate` or let Atlas Operator apply automatically

2. **Adding/Modifying Queries**:
   - Edit SQL queries in `queries/messages.sql`
   - Run `make generate` to regenerate Go code in `internal/db/`
   - Queries use sqlc annotations (`:exec`, `:many`, `:one`)

3. **Testing Locally**:
   - Use `make dev` for hot-reload development
   - Set environment variables via `.env` file (loaded when `Debug = true`)
   - Required env vars: `MYSQL_ADDRESS`, `MYSQL_USER`, `MYSQL_PASSWORD`
   - Optional: `PYROSCOPE_ENDPOINT`, `OTEL_EXPORTER_OTLP_ENDPOINT`

4. **Building Container**:
   - Multi-stage Dockerfile using Go 1.24 and distroless base
   - Builds static binary with CGO_ENABLED=0
   - Runs as non-root user (65532)

## Key Configuration Flags

- `--address` - HTTP listen address (default: `0.0.0.0:8080`)
- `--storage` - Storage type, only `mysql` supported
- `--mysql-address` - MySQL server address (required)
- `--mysql-database` - Database name (default: `slack_logger`)
- `--mysql-user` - Database user (required)
- `--mysql-password` - Database password (required)
- `--termination-grace-period` - Graceful shutdown timeout (default: 10s)
- `--lameduck` - Pre-shutdown delay for request draining (default: 1s)
- `--max-connections` - Maximum concurrent connections (default: 65532)

## Important Implementation Notes

- The service implements lameduck shutdown: it waits briefly after receiving SIGTERM before closing connections
- All messages are encrypted with channel-specific keys derived from channel IDs
- The service never decrypts messages during storage - decryption happens in retrieval APIs
- Thread messages are linked via `thread_ts` field
- Time-based queries support microsecond precision with inclusive/exclusive bounds
- The service recovers from panics in request handlers and returns 500 errors
