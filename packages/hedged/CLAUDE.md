# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Hedged is a Rust library that implements the [hedged requests pattern](https://cacm.acm.org/magazines/2013/2/160173-the-tail-at-scale/fulltext) for reducing tail latency in distributed systems. It sends duplicate requests with staggered delays and returns the first successful response.

## Common Development Commands

- `cargo fmt` - Format code
- `cargo clippy --fix` - Lint and fix issues  
- `cargo test` - Run tests
- `cargo build` - Build the library
- `cross build --target x86_64-unknown-linux-gnu` - Cross-compile for Linux GNU
- `cross build --target x86_64-unknown-linux-musl` - Cross-compile for Linux musl

## High-Level Architecture

### Core Functionality

The library exposes a single public async function `spawn` in src/lib.rs that:

1. **Takes three parameters:**
   - `timeout: Duration` - Delay between hedged requests
   - `count: u64` - Maximum number of hedged requests to spawn
   - `f: F` - The async function to execute (must return `Result<T, E>`)

2. **Creates staggered futures:**
   - First request executes immediately
   - Subsequent requests are delayed by `timeout × n` where n is the request number
   - All futures are wrapped in `tokio::time::timeout` with their respective delays

3. **Returns the first successful result:**
   - Uses `futures::future::select_all` to race all requests
   - Returns immediately when any request succeeds
   - Pending requests are automatically cancelled

### Usage Pattern

```rust
use hedged::spawn;
use std::time::Duration;

let result = spawn(
    Duration::from_millis(100),  // 100ms between hedged requests
    3,                           // Max 3 requests
    || async { make_request().await }
).await;
```

This pattern is particularly useful for reducing p99 latency when some requests may be slow due to network issues, GC pauses, or other transient problems.