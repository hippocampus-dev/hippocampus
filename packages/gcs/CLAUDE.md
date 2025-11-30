# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`gcs` is a tokio-based Google Cloud Storage client library with built-in authentication, retries, and hedged requests support. It's part of the larger Hippocampus workspace located at `/opt/hippocampus/packages/`.

## Common Development Commands

### From the root directory (`/opt/hippocampus/`):
- `make dev` - Main development command with auto-rebuild using watchexec
- `make fmt` - Format all Rust code
- `make lint` - Lint and auto-fix code with cargo clippy
- `make test` - Run all tests
- `make targets` - Cross-compile for Linux targets (x86_64-gnu and x86_64-musl)

### Package-specific commands:
- `cargo build` - Build the library
- `cargo test` - Run tests
- `cargo build --features use-openssl --no-default-features` - Build with OpenSSL instead of rustls

### Testing with Storage Emulator:
- Set `STORAGE_EMULATOR_HOST` environment variable to test against local storage emulator

## High-Level Architecture

### Core Components:
1. **Client struct** - Main entry point providing GCS operations with connection pooling
2. **Builder pattern** - Configurable client construction with options for TLS backend, retries, and hedging
3. **Authentication** - JWT-based Google service account authentication with automatic token refresh and caching
4. **Retry mechanism** - Built-in exponential backoff retry with configurable strategies
5. **Hedged requests** - Parallel request capability for improved reliability

### Key Design Patterns:
- **TLS Backend Flexibility**: Supports both rustls (default) and OpenSSL via feature flags
- **Error Classification**: Custom error types that classify retriability of operations
- **Streaming Support**: Built on hyper 0.14 with full HTTP/1 and HTTP/2 streaming capabilities
- **Observability**: Integrated tracing and OpenTelemetry support

### Dependencies on Internal Packages:
- `jwt` - JWT signing for Google OAuth2
- `elapsed` - Request timing utilities
- `error` - Shared error handling
- `retry` - Retry logic implementation
- `hedged` - Hedged request functionality

This library is designed to be a robust, production-ready GCS client with emphasis on reliability through retries and hedged requests.