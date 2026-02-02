# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Armyknife is a CLI utility tool written in Go that provides various development and operational utilities. It follows a Swiss Army knife design pattern with multiple subcommands for different tasks.

## Common Development Commands

### Building
```bash
# Standard build (without sqlite-vec support)
go build -o armyknife main.go

# Build with sqlite-vec support (requires CGO)
CGO_ENABLED=1 go build -o armyknife main.go

# Cross-platform builds (without sqlite-vec support)
GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -o armyknife main.go
```

**Note**: The `searchx` command requires CGO to be enabled for sqlite-vec support. When CGO is disabled, the command will return an error message indicating that CGO_ENABLED=1 is required.

### Testing
```bash
go test ./...
go test -v ./internal/rails_message_encryptor/
go test -v ./internal/marshal/ruby/
```

### Running
```bash
./armyknife <command> [options]
./armyknife --help  # See all available commands
```

### Dependencies
```bash
go mod tidy
go mod download
```

## High-Level Architecture

### Command Structure
The project follows a consistent pattern for implementing commands:

1. **Command Definition**: `/cmd/<command>.go` - Defines the Cobra command
2. **Arguments**: `/pkg/<command>/args.go` - Struct definitions for command arguments with validation tags
3. **Implementation**: `/pkg/<command>/<command>.go` - Core business logic

### Available Commands
- `bakery` - Cookie authentication service client
- `completion` - Shell completion generation for bash/zsh/fish
- `echo` - Echo server functionality
- `egosearch` - Slack message search with fuzzy finding
- `grpc` - gRPC utilities with call/catch subcommands
- `llm` - LLM integration (fillcsv for CSV processing via OpenAI)
- `mcp-notify` - MCP server for desktop notifications
- `proxy` - TCP/HTTP reverse proxy implementation
- `rails` - Rails credentials management (show/edit/diff)
- `s3` - S3 object viewer with fuzzy search
- `searchx` - Vector search functionality using SQLite-vec (requires CGO_ENABLED=1)
- `registry` - Observability registry management for Kubernetes manifests (generate `.registry.yaml`)
  - Subcommand `generate` - Generates `.registry.yaml` from Kubernetes manifests with Grafana Explore links
- `selfupdate` - Auto-update functionality via GitHub releases
- `serve` - Static file server

### Registry Generate Command

The `registry generate` command parses Kubernetes manifests to produce a `.registry.yaml` file containing observability metadata and Grafana Explore links.

#### Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--manifest` | `-m` | (required) | Path to manifest directory |
| `--output` | `-o` | `<manifest>/.registry.yaml` | Output file path |
| `--grafana-url` | | `https://grafana.minikube.127.0.0.1.nip.io` | Grafana base URL for generating Explore links |
| `--loki-datasource-uid` | | `loki` | Loki datasource UID for log Explore links |
| `--tempo-datasource-uid` | | `tempo` | Tempo datasource UID for trace Explore links |
| `--pyroscope-datasource-uid` | | `pyroscope` | Pyroscope datasource UID for profiling Explore links |

#### Output Format

The generated `.registry.yaml` includes:

- **logs** section: Always present if service has identifiable labels and namespace
  - `grouping`: Log grouping identifier (e.g., `kubernetes.<namespace>.<name>`)
  - `link`: Grafana Explore URL for logs using Loki datasource
- **traces** section: Only present if service defines `OTEL_EXPORTER_OTLP_ENDPOINT`
  - `serviceName`: OpenTelemetry service name from `OTEL_SERVICE_NAME`
  - `link`: Grafana Explore URL for traces using Tempo datasource
- **profiling** section: Only present if service defines `PYROSCOPE_ENDPOINT`
  - `serviceName`: Pyroscope application name
  - `link`: Grafana Explore URL for profiling using Pyroscope datasource
- **metrics** section: Present when labels and namespace are identified
  - `link`: Grafana workload dashboard URL (when workload type is detected from Deployment/StatefulSet/DaemonSet kind)
  - `labelSets`: List of label sets for metrics queries, each containing:
    - `labels`: Key-value map of Prometheus label matchers
    - `queries` (optional): Predefined Istio query templates (only present when `telemetry.yaml` exists in the manifest directory). Templates use `<<.LabelMatchers>>` placeholder for label substitution

#### Example Usage

```bash
# Generate with default Grafana settings
./armyknife registry generate --manifest cluster/manifests/bakery/overlays/dev

# Generate with custom Grafana URL
./armyknife registry generate \
  --manifest cluster/manifests/bakery/overlays/dev \
  --grafana-url https://grafana.kaidotio.dev \
  --output custom-registry.yaml
```

### Key Design Patterns

1. **Modular Command Registration**: Commands are registered in `/cmd/root.go` using a clean pattern where each command package exports a `GetRootCmd()` function.

2. **Validation**: Uses `github.com/go-playground/validator/v10` with struct tags for input validation.

3. **Error Handling**: Consistent use of `golang.org/x/xerrors` for wrapped errors with context.

4. **Protocol Buffers**: Uses protobuf definitions in `/armyknifepb/` for structured data exchange.

5. **External Service Integration**: Integrates with multiple services (AWS S3, Slack, OpenAI, GitHub) using their respective SDK clients.

### Package Organization
- `/cmd/` - Command-line interface definitions using Cobra
- `/pkg/` - Public packages containing business logic for each command
- `/internal/` - Internal packages for shared utilities (Rails encryption, Ruby marshaling, OpenAI client, registry parser)
- `/armyknifepb/` - Protocol buffer definitions and generated code
