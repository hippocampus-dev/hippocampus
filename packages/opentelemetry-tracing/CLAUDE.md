# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Package Overview

The `opentelemetry-tracing` package is a Rust library that provides custom logging macros (`trace!`, `debug!`, `info!`, `warn!`, `error!`) that automatically inject OpenTelemetry trace context (trace ID and span ID) into log events. This ensures distributed tracing correlation between logs and traces.

## Development Commands

### Building
```bash
# Build the library
cargo build

# Build in release mode
cargo build --release

# Cross-compile for specific targets (from root directory)
cross build --target x86_64-unknown-linux-gnu
cross build --target x86_64-unknown-linux-musl
```

### Testing
```bash
# Run tests
cargo test

# Run tests with release optimizations
cargo test --release
```

### Code Quality
```bash
# Format code
cargo fmt

# Lint code with auto-fix
cargo clippy --fix

# Check for unused dependencies
cargo udeps --all-targets --all-features
```

## Architecture

### Core Components

1. **Macro Support Module** (`__macro_support`):
   - Hidden module that extracts OpenTelemetry trace context from the current tracing span
   - `__traceparent()` function returns trace ID, span ID, and trace flags

2. **Logging Macros**:
   - Re-exports of standard tracing macros (`trace!`, `debug!`, `info!`, `warn!`, `error!`)
   - Each macro automatically injects `traceid` and `spanid` fields into events
   - Supports both simple and structured field syntax

### Key Design Decisions

- The library wraps standard `tracing` macros to ensure trace context is always included
- Uses `tracing-opentelemetry` to bridge between the tracing ecosystem and OpenTelemetry
- Trace and span IDs are converted to strings for compatibility with various log backends
- The `__macro_support` module is marked as `#[doc(hidden)]` to keep implementation details private

### Dependencies

- `tracing`: Core tracing framework
- `tracing-opentelemetry`: OpenTelemetry integration for tracing
- `opentelemetry`: OpenTelemetry API implementation

## Usage Pattern

Users of this library should use the macros from this crate instead of the standard `tracing` macros to ensure trace context propagation:

```rust
use opentelemetry_tracing::{info, error};

// Simple usage
info!("Processing request");

// With structured fields
error!({ error = %e, user_id = 123 }, "Request failed");
```