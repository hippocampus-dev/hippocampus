# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

ConnectRacer is an eBPF-based network connection tracer that monitors TCP connections (IPv4/IPv6) in real-time. It tracks outbound connections to specific hosts/IPs and exports Prometheus metrics. The tool is Kubernetes-aware and designed for monitoring outbound connections in containerized environments.

## Common Development Commands

### Build and Development
- `make dev` - Run with watchexec and mold linker for hot-reload development
- `make all` - Complete build pipeline: format, lint, build, and test
- `make fmt` - Format code with cargo fmt
- `make lint` - Lint and fix issues with cargo clippy
- `make targets` - Cross-compile for x86_64-unknown-linux-gnu

### Testing and Running
- `cargo test` - Run unit tests
- `cargo run -- --help` - See CLI options
- Example run: `cargo run -- --address 127.0.0.1:8080 --nameserver 1.1.1.1:53 --hosts example.com`

### eBPF Development
- eBPF code is in `src/bpf/connect.bpf.c`
- Build system automatically compiles eBPF via `build.rs`
- Requires: clang, libelf-dev, bpftool installed

## High-Level Architecture

### Core Components

1. **eBPF Program** (`src/bpf/connect.bpf.c`)
   - Hooks into `tcp_v4_connect` and `tcp_v6_connect` kernel functions
   - Captures connection events with process info (PID, UID, command)
   - Filters connections by destination IP addresses
   - Sends events via perf buffer to userspace

2. **Rust Application Structure**
   - `main.rs` - Orchestrates DNS resolution, IP monitoring, and eBPF lifecycle
   - `bpf.rs` - Manages eBPF program loading and event processing
   - `server.rs` - HTTP server exposing `/metrics` for Prometheus
   - `metadata/kubernetes.rs` - Extracts container IDs from cgroup paths

### Key Design Patterns

1. **Dynamic DNS Resolution**: Monitors configured hostnames and updates IP filters when DNS changes
2. **TTL-Aware Caching**: Respects DNS TTLs for efficient resolution
3. **Async Event Processing**: Uses Tokio for concurrent DNS resolution and event handling
4. **OpenTelemetry Integration**: Structured metrics with labels (host, port, command, container_id)

### Development Notes

- The project uses Rust 1.89.0 (see `rust-toolchain.toml`)
- Cross-compilation configured in `Cross.toml` for Linux targets
- Multi-stage Dockerfile for optimized container builds
- Integrates with the Hippocampus monorepo build system
