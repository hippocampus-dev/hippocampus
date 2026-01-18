# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`serde_binary` is a Rust library that provides binary serialization and deserialization capabilities using the Serde framework. It implements custom serializers and deserializers for encoding data in a compact binary format.

## Common Development Commands

- `cargo fmt` - Format code according to Rust style guidelines
- `cargo clippy --fix` - Lint code and automatically fix issues
- `cargo test` - Run all tests
- `cargo build` - Build the library
- `cargo build --release` - Build optimized release version
- `cargo doc --open` - Generate and view documentation

## High-Level Architecture

### Core Components

1. **Error Handling** (`lib.rs`):
   - Custom `Error` enum with `Message` and `EOF` variants
   - Implements standard error traits and Serde error traits

2. **Serialization** (`ser.rs`):
   - `Serializer` struct that writes to any `std::io::Write` implementation
   - Implements Serde's `Serializer` trait with partial support for various data types
   - Currently supports: strings, bytes, maps, structs, newtype structs/variants
   - Helper functions: `to_writer()` and `to_vec()`

3. **Deserialization** (`de.rs`):
   - `Deserializer` struct that reads from any `std::io::Read` implementation
   - Implements Serde's `Deserializer` trait with partial support
   - Currently supports: strings, maps, structs, tuples, enums, newtype structs
   - Helper function: `from_slice()`

### Binary Format

The library uses big-endian encoding:
- Strings/bytes: 8-byte length prefix (usize as BE) followed by raw data
- Maps: 8-byte length prefix indicating number of key-value pairs
- Enums: 4-byte variant index (u32 as BE) followed by variant data

### Implementation Status

Many primitive types and complex structures are currently marked as `unimplemented!()`, indicating this is a work-in-progress library focused on specific use cases within the Hippocampus project.