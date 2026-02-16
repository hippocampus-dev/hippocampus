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

For commands with subcommands, the pattern nests one level deeper:

1. **Command Definition**: `/cmd/<command>/<subcommand>.go` - Defines each subcommand
2. **Arguments**: `/pkg/<command>/<subcommand>/args.go` - Struct definitions for subcommand arguments
3. **Implementation**: `/pkg/<command>/<subcommand>/<subcommand>.go` - Core business logic

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
- `registry` - Observability registry management for Kubernetes manifests
  - Subcommand `generate` - Generates `.registry.yaml` from Kubernetes manifests with Grafana Explore links
  - Subcommand `query` - Queries observability data from Grafana datasources using `.registry.yaml` files
- `selfupdate` - Auto-update functionality via GitHub releases
- `serve` - Static file server

### Registry Generate Command

The `registry generate` command parses Kubernetes manifests to produce a `.registry.yaml` file containing observability metadata and Grafana Explore links.

#### Arguments

| Argument | Description |
|----------|-------------|
| `DIRECTORY` | Path to manifest directory |

#### Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--output` | `-o` | `<manifest>/.registry.yaml` | Output file path |
| `--grafana-url` | | `https://grafana.minikube.127.0.0.1.nip.io` | Grafana base URL for generating Explore links |
| `--loki-datasource-uid` | | `loki` | Loki datasource UID for log Explore links |
| `--tempo-datasource-uid` | | `tempo` | Tempo datasource UID for trace Explore links |
| `--pyroscope-datasource-uid` | | `pyroscope` | Pyroscope datasource UID for profiling Explore links |

#### Output Format

The generated `.registry.yaml` includes:

- **workloadType**: Kubernetes workload type (`deployment`, `statefulset`, `daemonset`). Present when the manifest kind is recognized
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
    - `queries` (optional): Predefined query templates using `<<.LabelMatchers>>` placeholder for label substitution. Istio query templates are present when `telemetry.yaml` exists in the manifest directory. Container resource query templates (CPU Usage, CPU Throttled, CPU Request Utilization, Memory Working Set, Memory Limit Utilization, Container Restarts, Network Receive, Network Transmit, Network Receive Dropped, Network Transmit Dropped) are present when workload type is detected, using `mixin_pod_workload` join for workload-level scoping with per-pod breakdown (`sum by (pod)`)

#### Example Usage

```bash
# Generate with default Grafana settings
./armyknife registry generate cluster/manifests/bakery/overlays/dev

# Generate with custom Grafana URL
./armyknife registry generate cluster/manifests/bakery/overlays/dev \
  --grafana-url https://grafana.kaidotio.dev \
  --output custom-registry.yaml
```

### Registry Query Command

The `registry query` command reads a `.registry.yaml` file from a specified directory and executes observability queries against Grafana datasources (Prometheus, Loki, Tempo, Pyroscope), outputting YAML results to stdout.

#### Arguments

| Argument | Description |
|----------|-------------|
| `DIRECTORY` | Directory containing `.registry.yaml` |

#### Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--grafana-url` | | `https://grafana.minikube.127.0.0.1.nip.io` | Grafana base URL |
| `--authorization-listen-port` | | `0` | Port to listen for authorization callback |
| `--prometheus-datasource-uid` | | `prometheus` | Prometheus datasource UID |
| `--loki-datasource-uid` | | `loki` | Loki datasource UID |
| `--tempo-datasource-uid` | | `tempo` | Tempo datasource UID |
| `--pyroscope-datasource-uid` | | `pyroscope` | Pyroscope datasource UID |
| `--from` | | `30m` | Duration lookback from now |
| `--to` | | `0s` | Duration offset from now for end time |
| `--step` | | `30s` | Prometheus query step interval |
| `--signals` | | (all) | Signal types to query (metrics,logs,traces,profiling) |

#### Query Behavior

- Reads `<directory>/.registry.yaml` directly (no recursive directory walk)
- For each component in the registry, queries each signal type present (filtered by `--signals` if specified, all signals when omitted):
  - **metrics**: Expands query templates with label matchers and queries Prometheus
  - **logs**: Queries Loki using the log grouping identifier
  - **traces**: Queries Tempo using the service name
  - **profiling**: Queries Pyroscope using the service name
- Time range is calculated as `[now - to - from, now - to]` (e.g., `--from 6h --to 1h` queries from 7h ago to 1h ago)
- Results are output as YAML to stdout
- Optional OAuth2 authentication for Grafana via Bakery (hardcoded URL, enabled by setting `--authorization-listen-port`)

#### Output Format

Results are output per signal type:

| Signal | Output Fields |
|--------|---------------|
| **metrics** | Per-series: `labels`, `values` (array of `[timestamp_ms, value]` pairs). NaN/Inf values are skipped |
| **logs** | `lines` (all log lines with RFC3339 `timestamp` and `body`) |
| **traces** | `traces` (all traces with `traceId`, `name`, human-readable `duration`) |
| **profiling** | Same raw time series format as metrics |

- Metric and profiling series contain raw time series data points as `[][2]float64` where each entry is `[timestamp_ms, value]`
- Trace durations are formatted as human-readable strings (e.g., `1.23s`, `45.67ms`, `123.45us`), with NaN/Inf/negative values shown as `N/A`
- Log timestamps are formatted as RFC3339 in UTC

#### Example Usage

```bash
# Query a specific manifest's registry
./armyknife registry query cluster/manifests/bakery/overlays/dev

# Query with custom time range (from 6h ago to 1h ago)
./armyknife registry query cluster/manifests/bakery/overlays/dev \
  --from 6h \
  --to 1h

# Query only metrics and logs signals
./armyknife registry query cluster/manifests/bakery/overlays/dev \
  --signals metrics,logs

# Query with Bakery authentication
./armyknife registry query cluster/manifests/bakery/overlays/dev \
  --authorization-listen-port 18080
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
