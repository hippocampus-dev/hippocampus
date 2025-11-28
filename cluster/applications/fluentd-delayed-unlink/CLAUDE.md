# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

fluentd-delayed-unlink is an eBPF-based system tool that prevents race conditions between Kubernetes log rotation and Fluentd log processing. It intercepts file unlink operations using eBPF programs and delays them until Fluentd has finished processing the logs, preventing data loss.

## Common Development Commands

### Development
- `make dev` - Runs watchexec with cargo build for hot-reload development
- `make all` - Full build pipeline: format, lint, and cross-compile for Linux targets
- `make fmt` - Format code using cargo fmt
- `make lint` - Run linting via serial-makefile

### Building
- `cross build -q --release --target x86_64-unknown-linux-gnu` - Build for Linux x86_64
- `make targets` - Build for all configured target architectures
- Docker build available via Dockerfile for containerized deployment

### Testing
- `cross test -q --release --target x86_64-unknown-linux-gnu` - Run tests for specific target

## High-Level Architecture

### Core Components

1. **eBPF Layer** (`src/bpf/unlink.bpf.c`)
   - Kernel-space C program that hooks into `unlink` and `unlinkat` syscalls
   - Filters operations based on target directory path
   - Sends events to userspace for processing

2. **Rust Application** (`src/main.rs`)
   - Loads and manages eBPF programs using libbpf-rs
   - Processes unlink events from kernel space
   - Checks Fluentd position file before allowing deletions
   - Implements delay mechanism for file deletions

3. **Metrics Server** (`src/server.rs`)
   - HTTP server on configurable address (default: 127.0.0.1:8080)
   - Exposes Prometheus metrics for monitoring delayed unlinks
   - Provides health/readiness endpoints

### Key Design Patterns

- **Async Runtime**: Uses Tokio for concurrent operations and HTTP server
- **Cross-thread Communication**: mpsc channels between eBPF watcher thread and main async runtime
- **Metrics**: OpenTelemetry with Prometheus exporter for observability
- **Cross-compilation**: Supports building for Linux targets from any platform using `cross`

### Command-line Arguments
- `--address` - HTTP server address (default: 127.0.0.1:8080)
- `-d, --directory` - Directory to monitor (default: /var/log/containers)
- `-f, --pos-file` - Fluentd position file path (default: /var/log/fluentd-containers.log.pos)
- `-s, --delayed-seconds` - Delay in seconds before allowing unlink (default: 1)

### Dependencies
- **Runtime**: libelf (required for eBPF operations)
- **Build**: clang, bpftool for eBPF compilation
- **Development**: watchexec, mold linker for faster builds