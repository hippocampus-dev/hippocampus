# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

The `singleflight` crate provides call deduplication for concurrent async operations. When multiple callers request the same key concurrently, only one executes the underlying function while others wait and receive a clone of the result.

## Common Development Commands

- `cargo fmt` - Format code
- `cargo clippy --fix` - Lint and fix issues
- `cargo test` - Run tests
- `cargo build` - Build the library
- `cross build --target x86_64-unknown-linux-gnu` - Cross-compile for Linux GNU
- `cross build --target x86_64-unknown-linux-musl` - Cross-compile for Linux musl

## High-Level Architecture

### Core Functionality

The library exposes a `Group<K, V>` struct in src/lib.rs with a single public method `work`:

1. **Takes two parameters:**
   - `key: &K` - The deduplication key
   - `f: F` - The async function to execute (called once per concurrent group)

2. **Deduplication mechanism:**
   - Uses `std::sync::Mutex<HashMap<K, Arc<Flight<V>>>>` to track in-flight requests
   - `Flight` wraps a `tokio::sync::OnceCell<V>` and a `duplicates: AtomicUsize` counter
   - First caller becomes leader and executes the function
   - Subsequent callers for the same key wait on the same `OnceCell` and increment `duplicates`
   - Returns `(V, bool)` where the bool indicates if the result was shared across multiple callers

3. **Cancellation safety:**
   - If the leader is cancelled, `tokio::sync::OnceCell` promotes the next waiter
   - Cleanup uses `Arc::ptr_eq` to avoid removing newer flights

### Usage Pattern

```rust
use singleflight::Group;

let group: Group<String, Result<Token, std::sync::Arc<Error>>> = Group::new();

let (result, shared) = group.work(&key, || async {
    fetch_token().await
}).await;
```

## Dependencies

- `tokio` (1.45.1): Async runtime for `OnceCell` synchronization primitive
- `std::sync::atomic::AtomicUsize`: Tracks duplicate callers per flight
