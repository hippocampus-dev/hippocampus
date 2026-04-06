# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

The `swr` crate provides a Stale-While-Revalidate cache for async operations. When cached data becomes stale (past the soft TTL), it returns the stale value immediately while triggering a background refresh. When data expires (past the hard TTL), it blocks on a fresh fetch. Uses [`singleflight`](../singleflight) to deduplicate concurrent refresh operations.

## Common Development Commands

- `cargo fmt` - Format code
- `cargo clippy --fix` - Lint and fix issues
- `cargo test` - Run tests
- `cargo build` - Build the library
- `cross build --target x86_64-unknown-linux-gnu` - Cross-compile for Linux GNU
- `cross build --target x86_64-unknown-linux-musl` - Cross-compile for Linux musl

## High-Level Architecture

### Core Functionality

The library exposes a `Cache<K, V, E>` struct in src/lib.rs with a single public method `get`:

1. **Takes two parameters:**
   - `key: &K` - The cache key
   - `f: F` - The async function to fetch a fresh value, returning `Result<FetchResult<V>, E>` where `FetchResult` contains `value`, `stale_after`, and `expire_after`

2. **Three cache states:**
   - Fresh (`elapsed < stale_after`): Return cached value immediately
   - Stale (`stale_after <= elapsed < expire_after`): Return cached value, spawn background refresh via `singleflight`
   - Expired (`elapsed >= expire_after`) or miss: Block on fetch via `singleflight`

3. **Background refresh deduplication:**
   - Uses `singleflight::Group` so concurrent stale hits trigger only one background fetch
   - Background refresh runs in a `tokio::spawn` task

### Usage Pattern

```rust
use swr::Cache;

let cache: Cache<String, String, MyError> = Cache::new();

let value = cache.get(
    &"key".to_string(),
    || async {
        let data = fetch_from_backend().await?;
        Ok(swr::FetchResult {
            value: data,
            stale_after: Duration::from_secs(60),
            expire_after: Duration::from_secs(300),
        })
    },
).await?;
```

## Dependencies

- `tokio` (1.45.1): Async runtime for `Instant`, `spawn`, and synchronization
- `singleflight`: Call deduplication for concurrent refresh operations
