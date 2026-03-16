# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Redis proxy written in Go that provides connection pooling, read/write splitting, and topology-aware routing. It acts as an intermediary between clients and Redis servers, managing connections efficiently and routing commands based on their type.

## Common Development Commands

### Development
- `make dev` - Runs the proxy locally with watchexec for auto-reload on code changes
  - Default configuration: listens on :16379, connects to Redis on :6379, metrics on :8080
  - Supports hot-reload during development

### Testing
- `go test` - Run unit tests
- `go test -bench=.` - Run benchmarks

### Building
- `go build -o redis-proxy main.go` - Build the binary locally
- `docker build -t redis-proxy .` - Build the Docker image

## Architecture

### Core Components

1. **RESP Protocol Parser** - Implements Redis Serialization Protocol (RESP) parsing
   - Handles all RESP data types: Simple Strings, Errors, Integers, Bulk Strings, Arrays
   - Used for inspecting commands to enable read/write splitting

2. **Connection Pool** - Manages TCP connections to Redis servers
   - Configurable pool size, idle connections, and lifetime limits
   - Supports FIFO/LIFO strategies
   - Health checking via syscall-level connection inspection
   - Jitter support for connection lifetime to avoid thundering herd

3. **Command Router** - Routes commands based on type
   - Read commands can be routed to separate reader endpoints
   - Write commands go to the primary Redis instance
   - Command classification in `isReadCommand()` function

4. **Topology-Aware Routing** - Routes connections based on network topology
   - Supports CIDR-based topology definitions
   - Prefers connections within the same topology/zone

### Key Design Patterns

- **Zero-copy proxying** - Uses `io.Copy` for efficient data transfer
- **Graceful shutdown** - Handles SIGTERM with configurable grace period and lameduck mode
- **Metrics** - Prometheus metrics via OpenTelemetry for connection counts and command rates
- **Connection reuse** - Returns healthy connections to the pool after use

### Command-Line Flags

Critical flags for operation:
- `--local-address` - Address to listen on (default: 127.0.0.1:16379)
- `--remote-address` - Primary Redis server address (default: 127.0.0.1:6379)
- `--reader-routing` - Enable read/write splitting
- `--reader-remote-address` - Redis reader endpoint for read commands
- `--max-idle-connections` - Maximum idle connections in pool
- `--min-idle-connections` - Minimum idle connections to maintain
- `--topology-aware-routing` - Enable topology-aware connection routing
- `--topologies` - Topology definitions (format: name1=CIDR1,name2=CIDR2)