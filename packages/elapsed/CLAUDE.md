# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

The `elapsed` crate provides a serializable wrapper around `std::time::Instant` to enable elapsed time calculations across serialization boundaries. This is particularly useful in distributed systems where timing information needs to be passed between processes.

## Common Development Commands

- `cargo fmt` - Format code
- `cargo clippy --fix` - Lint and fix issues  
- `cargo test` - Run tests
- `cargo build` - Build the library
- `cargo doc --open` - Build and view documentation
- `cargo build --release` - Build optimized version
- `cross build --target x86_64-unknown-linux-gnu` - Cross-compile for Linux (GNU)
- `cross build --target x86_64-unknown-linux-musl` - Cross-compile for Linux (musl)

## Architecture

### Core Components

1. **`SerializableTime`** - Main struct that wraps `std::time::Instant` with custom serialization
   - Converts between `Instant` and `SystemTime` for serialization
   - Preserves elapsed time calculations across serialization boundaries

2. **`approx_instant` module** - Custom serde serialization/deserialization implementation
   - Serializes by converting to `SystemTime` relative to current time
   - Deserializes by reconstructing `Instant` from elapsed duration

3. **Integration with `elapsed_macro`** - This crate works in conjunction with the `elapsed_macro` procedural macro that provides the `#[elapsed]` attribute for automatic execution time measurement

### Key Design Decisions

- Uses `SystemTime` as an intermediate representation since `Instant` cannot be directly serialized
- Approximates the original `Instant` by calculating elapsed time and working backwards from current time
- Exposes both the macro and `serde_json` through the prelude for convenience

## Usage Example

```rust
use elapsed::prelude::*;
use elapsed::SerializableTime;

// Create a serializable time instance
let start = SerializableTime::new(std::time::Instant::now());

// Serialize to JSON
let json = serde_json::to_string(&start).unwrap();

// Deserialize back
let deserialized: SerializableTime = serde_json::from_str(&json).unwrap();

// Calculate elapsed time
let duration = deserialized.elapsed();
```