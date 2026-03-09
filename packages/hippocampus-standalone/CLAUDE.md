# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Hippocampus Standalone is a terminal-based client for the Hippocampus search and indexing system. It provides command-line interfaces for indexing documents and searching through them using TF-IDF scoring.

## Commands

### Build Commands
- `cargo build` - Build the standalone client
- `cargo build --release` - Build optimized release version
- `cross build --bin hippocampus-standalone --release --target x86_64-unknown-linux-gnu` - Cross-compile for Linux

### Run Commands
- `cargo run -- -c hippocampus.toml index [FILES...]` - Index files
- `cargo run -- -c hippocampus.toml search -i` - Interactive search mode
- `cargo run -- -c hippocampus.toml search "query"` - Non-interactive search

### Testing
- `cargo test` - Run tests
- `cargo test --features tracing` - Run tests with tracing enabled

### Development
- From project root: `make dev` - Auto-rebuild on file changes using watchexec
- `cargo fmt` - Format code
- `cargo clippy --fix` - Lint and auto-fix issues

## Architecture

### Key Components

1. **Storage Configuration** (via `hippocampus-configuration` crate)
   - Supports File, SQLite (with feature), GCS, and Cassandra (with feature) backends
   - Configured via `hippocampus.toml`
   - Token storage for inverted index
   - Document storage for original documents

2. **Main Entry Point** (`main.rs`)
   - CLI interface using Clap
   - Two subcommands: `index` and `search`
   - OpenTelemetry tracing support (optional)
   - Supports both jemalloc and system allocator

3. **Interactive UI** (`ui.rs`)
   - Terminal-based search interface using termion
   - Real-time search results as you type
   - Navigate results with arrow keys

### Storage Backends

The application supports hybrid storage configurations:
- **DocumentStorage**: File, SQLite (with `sqlite` feature)
- **TokenStorage**: File, SQLite (with `sqlite` feature), GCS, Cassandra (with `cassandra` feature)

### Features

- `default`: Uses rustls and ipadic dictionary
- `use-rustls`/`use-openssl`: TLS implementation choice
- `ipadic`/`ko-dic`: Language-specific tokenization dictionaries
- `tracing`: OpenTelemetry tracing support
- `jemalloc`: Use jemalloc instead of system allocator
- `sqlite`: Enable SQLite storage backend
- `cassandra`: Enable Cassandra storage backend
- `wasm`: Enable WASM tokenizer support

### Dependencies

Key external crates:
- `hippocampus-core`: Core indexing/search functionality
- `hippocampusql`: Query language parser
- `lindera`: Japanese/Korean tokenizer
- `termion`: Terminal UI
- `tokio`: Async runtime
- `opentelemetry`: Distributed tracing

## Configuration

The `hippocampus.toml` file configures storage, tokenizer, and schema:

```toml
[TokenStorage]
kind = "File"
path = "/var/hippocampus/tokens"

[DocumentStorage]
kind = "File"
path = "/var/hippocampus/documents"

[Tokenizer]
kind = "Lindera"

[Schema]
fields = [
    { name = "file", type = "string", indexed = false },
    { name = "content", type = "string", indexed = true },
]
```

## Usage Notes

- The indexer processes files concurrently (default: 10 concurrent files)
- Search results include TF-IDF scores and text fragments
- Interactive mode provides a better search experience than command-line queries
- GCS backend requires proper service account credentials with storage permissions