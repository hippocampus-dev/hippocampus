# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

The `error` package is a minimal Rust error handling library that provides a drop-in replacement for `Box<dyn std::error::Error + Send + Sync + 'static>` with built-in support for `std::backtrace` and context attachment.

## Common Development Commands

### Building
```bash
cargo build
```

### Testing
```bash
cargo test
```

### Formatting
```bash
cargo fmt
```

### Linting
```bash
cargo clippy
```

## High-Level Architecture

### Core Components

1. **`Error` Type** (`src/lib.rs:64-215`): The main error type that wraps any `std::error::Error` and optionally captures backtraces. It provides:
   - Automatic conversion from any error type that implements `std::error::Error + Send + Sync + 'static`
   - Downcasting support for error inspection
   - Backtrace capture when available

2. **Context System** (`src/context.rs`): Provides two traits for adding context to errors:
   - `ContextableError`: For adding context to any error type
   - `Context`: Extension trait for `Result` types to easily add context with `?`

3. **Error Construction Macros**:
   - `error!`: Creates an `error::Error` from a format string
   - `bail!`: Early returns with an error (shorthand for `return Err(error!(...))`)

### Key Design Decisions

- The library captures backtraces automatically when `RUST_BACKTRACE` is enabled
- All errors are stored as trait objects to maintain flexibility
- Context is added by wrapping errors in a `ContextError` type that preserves the chain
- The library is designed to work seamlessly with the `?` operator for error propagation