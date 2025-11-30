# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Package Overview

Bakery is a Rust library for OAuth-style cookie authentication/authorization flows. It provides a browser-based authentication client that manages cookies with automatic expiration handling.

## Common Development Commands

- `cargo fmt` - Format code
- `cargo clippy --fix` - Lint and fix issues  
- `cargo test` - Run tests
- `cargo build` - Build library
- `make dev` (from workspace root) - Watch mode with auto-rebuild

## Architecture

### Core Components

**`Client` struct** - Main API surface with these key methods:
- `get_value()` - Returns valid cookie value, triggering re-authentication if expired
- `challenge()` - Initiates browser authentication flow:
  1. Spawns local HTTP server on random port
  2. Opens browser to auth URL with callback
  3. Receives cookie from callback
  4. Persists to disk
- `save()`/`restore()` - Cookie persistence to `~/.local/share/bakery/`

### Key Design Decisions

1. **Browser-based auth**: Uses `webbrowser` crate to open system browser for user authentication
2. **Local callback server**: Hyper-based HTTP server receives OAuth callbacks on ephemeral ports
3. **XDG compliance**: Stores cookies in platform-appropriate data directories
4. **Automatic expiration**: Checks cookie validity before returning, re-authenticates if needed
5. **Async-first**: Built on Tokio runtime for non-blocking operations

### Dependencies

- `tokio` - Async runtime
- `hyper` - HTTP server for OAuth callbacks
- `chrono` - Cookie expiration handling
- `serde_json` - Cookie serialization
- `url` - URL manipulation for auth flows
- `webbrowser` - Cross-platform browser launching