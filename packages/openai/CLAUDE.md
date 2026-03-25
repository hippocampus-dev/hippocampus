# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is the `openai` package within the Hippocampus monorepo. It provides a Rust HTTP client for interacting with OpenAI-compatible APIs, with support for chat completions and realtime API features.

## Common Development Commands

### Building and Testing
- `cargo build` - Build the package
- `cargo test` - Run tests
- `cargo fmt` - Format code
- `cargo clippy --fix` - Lint and auto-fix issues
- `cargo udeps --all-targets --all-features` - Check for unused dependencies

### Cross-compilation
- `cross build --target x86_64-unknown-linux-gnu` - Build for Linux GNU
- `cross build --target x86_64-unknown-linux-musl` - Build for Linux musl

## High-Level Architecture

### Core Components

1. **HTTP Client (`lib.rs`)**: 
   - Configurable client with retry strategies and connection pooling
   - Support for both rustls and OpenSSL TLS backends (feature-gated)
   - Built on hyper with OpenTelemetry tracing integration
   - Automatic retry logic for transient failures

2. **Type Definitions (`types/`)**:
   - `chat.rs`: Types for chat completions API (roles, messages, responses)
   - `realtime.rs`: Types for realtime API (events, sessions, tools)

### Key Design Patterns

1. **Feature Flags**:
   - `use-rustls` (default): Use rustls for TLS
   - `use-openssl`: Use OpenSSL for TLS
   - `tracing`: Enable tracing instrumentation

2. **Error Handling**:
   - Custom error types with retry classification
   - Integration with the `error` and `retry` workspace packages

3. **Configuration**:
   - Builder pattern for client configuration
   - Environment variable support (`OPENAI_BASE_URL`)
   - Configurable timeouts, connection pooling, and retry strategies

### Dependencies
- Uses workspace packages: `retry`, `error`
- External: `hyper`, `serde`, `tracing`, `futures`