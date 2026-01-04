# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`enum_derive` is a Rust procedural macro crate that provides derive macros to add utility methods to enums. It enables treating enums as iterators and provides conversion to string functionality. The crate is part of the larger Hippocampus workspace.

## Common Development Commands

### Building and Testing
- `cargo build` - Build the crate
- `cargo test` - Run the test suite in tests/main.rs
- `cargo fmt` - Format code according to Rust standards
- `cargo clippy --fix` - Lint and auto-fix issues

### Workspace Commands (from parent directory)
- `make all` - Runs formatting, linting, testing, and builds all targets
- `make fmt` - Format all Rust code in the workspace
- `make lint` - Lint all code with auto-fixes
- `make test` - Run all tests in the workspace

## High-Level Architecture

### Derive Macros Provided

1. **`EnumLen`** - Adds a `len()` method that returns the number of variants in the enum
2. **`EnumIter`** - Creates an iterator struct for the enum and adds an `iter()` method to iterate over all variants
3. **`EnumToString`** - Adds a `to_string()` method that converts the enum variant to its name as a string

### Key Implementation Details

- All macros only work with unit variants (enums without fields)
- `EnumIter` generates a separate iterator struct named `{EnumName}Iter`
- The generated iterator struct derives `Default` and uses an index-based approach
- All generated methods are added directly to the enum impl block
- Uses `syn` for parsing, `quote` for code generation, and `proc-macro2` for token manipulation

### Limitations

- Only supports enums with unit variants (no tuple or struct variants)
- Panics if applied to non-enum types or enums with non-unit variants
- Generated methods use simple names that could conflict with existing methods