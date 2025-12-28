# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

`elapsed_macro` is a procedural macro crate that provides the `#[elapsed]` attribute for measuring function execution time. It integrates with the `elapsed` crate to track timing from a base point stored in an environment variable.

## Development Commands

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

## Architecture

This is a proc-macro crate that:
1. Accepts functions annotated with `#[elapsed]`
2. Wraps the function body to measure elapsed time from a base timestamp
3. Uses environment variable `ELAPSED::BASE_TIME` to store/retrieve the base timing
4. Prints timing information showing how long after the base time the function was called

### Key Components
- **lib.rs**: Contains the single `elapsed` attribute macro that:
  - Parses the function AST using `syn`
  - Injects timing logic at the start of the function body
  - Uses `quote` to generate the modified function
  - Depends on the `elapsed` crate for `SerializableTime` type
  - Uses `serde_json` for serializing timing data to/from the environment variable

### Dependencies
- `syn` (v2.0.67): For parsing Rust syntax
- `quote` (v1.0.36): For generating Rust code
- External dependency on `elapsed` crate (provides `SerializableTime`)
- External dependency on `serde_json` (for serialization)