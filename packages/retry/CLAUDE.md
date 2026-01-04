# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Package Overview

The `retry` crate provides asynchronous retry functionality with configurable delay strategies, implementing exponential backoff with jitter. It's designed for handling transient failures in async operations like network requests or API calls.

## Common Development Commands

- `cargo build` - Build the crate
- `cargo test` - Run unit tests
- `cargo fmt` - Format code according to Rust standards
- `cargo clippy` - Run linter for code quality checks
- `cargo doc --open` - Generate and view documentation

## Code Architecture

### Core API

The main entry point is the `spawn` function in `src/lib.rs`:
```rust
pub async fn spawn<I, O, F, T>(strategy: I, mut f: F) -> Result<T, Error>
```

This function takes:
- A retry strategy that implements `Iterator<Item = Duration>`
- An async function that returns `Result<T, RetryError>`

### Error Handling

The crate uses a two-tier error system:
- `RetryError`: Distinguishes between retriable and unexpected errors
- `Error`: The final result type with `RetryExceeded` or `Unexpected` variants

Functions must wrap errors appropriately:
- Use `RetryError::from_retriable_error()` for transient failures
- Use `RetryError::from()` for permanent failures

### Retry Strategies

Three built-in strategies in `src/strategy.rs`:
- `NoDelay`: Immediate retries without delay
- `FixedDelay`: Constant delay between retries
- `JitteredExponentialBackoff`: Exponentially increasing delays with random jitter
  - Default base delay: 10ms, doubling each retry
  - Configurable max delay cap
  - Customizable jitter function

### Usage Pattern

```rust
let strategy = JitteredExponentialBackoff::new(Duration::from_millis(100))
    .take(5); // Limit to 5 retries

let result = spawn(strategy, || async {
    // Your async operation
    do_something().await
        .map_err(|e| RetryError::from_retriable_error(Box::new(e)))
}).await;
```

## Dependencies

- `tokio` (1.45.1): Async runtime for sleep functionality
- `rand` (0.9.0): Random number generation for jitter