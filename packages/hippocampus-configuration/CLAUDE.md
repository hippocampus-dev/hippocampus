# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

hippocampus-configuration is a shared configuration parsing library for hippocampus applications. It provides TOML configuration file parsing with feature-gated storage backends.

## Common Development Commands

- `cargo build` - Build the library
- `cargo test` - Run tests
- `cargo fmt` - Format code
- `cargo clippy --fix` - Lint and fix issues

## Architecture

### Feature Flags

Storage backends can be enabled via feature flags:

- `sqlite`: Enable SQLite storage backend (for both DocumentStorage and TokenStorage)
- `gcs`: Enable Google Cloud Storage backend (TokenStorage only)
- `cassandra`: Enable Cassandra storage backend (TokenStorage only)
- `wasm`: Enable WASM tokenizer support

### Configuration Structure

The configuration file format is compatible with both `hippocampus-server` and `hippocampus-standalone`:

```toml
[TokenStorage]
kind = "File"
path = "/var/hippocampus/tokens"

# SQLite example (requires sqlite feature):
# [TokenStorage]
# kind = "SQLite"
# path = "/var/hippocampus/tokens.db"

# GCS example (requires gcs feature):
# [TokenStorage]
# kind = "GCS"
# bucket = "my-bucket"
# prefix = "tokens"
# service_account_key_path = "/path/to/key.json"

# Cassandra example (requires cassandra feature):
# [TokenStorage]
# kind = "Cassandra"
# address = "127.0.0.1:9042"

[DocumentStorage]
kind = "File"
path = "/var/hippocampus/documents"

# SQLite example (requires sqlite feature):
# [DocumentStorage]
# kind = "SQLite"
# path = "/var/hippocampus/documents.db"

[Tokenizer]
kind = "Lindera"

# Wasm example (requires wasm feature):
# [Tokenizer]
# kind = "Wasm"
# path = "/path/to/tokenizer.wasm"

[Schema]

[[Schema.fields]]
name = "content"
type = "string"
indexed = true
```

### Storage Backend Support

| Backend   | DocumentStorage | TokenStorage |
|-----------|-----------------|--------------|
| File      | Yes             | Yes          |
| SQLite    | Yes (feature)   | Yes (feature)|
| GCS       | No              | Yes (feature)|
| Cassandra | No              | Yes (feature)|

### Key Components

- `src/lib.rs` - All configuration types and parsing logic

### Usage

```rust
// Load configuration from file
let config = hippocampus_configuration::Configuration::from_file(path)?;

// Access storage configurations
let doc_file_config = config.document_storage.file_configuration()?;
let token_gcs_config = config.token_storage.gcs_configuration()?;
```

## Dependencies

- `serde` - Serialization/deserialization
- `toml` - TOML parsing
- `error` - Error handling (workspace member)
