# Error Handling Details

Detailed guidelines for error classification and retry patterns.

## RetryError Classification

Use `RetryError` to classify errors from external operations:

| Classification | Use Case | Examples |
|----------------|----------|----------|
| `from_retriable_error` | Transient failures that may succeed on retry | Connection refused, timeout, 503 |
| `from_unexpected_error` | Permanent failures that won't succeed on retry | Auth failure, 404, validation error |

## Retry + Hedged Pattern

For resilient external calls, combine `retry::spawn` with `hedged::spawn`:

```rust
retry::spawn(self.retry_strategy.clone(), || async {
    hedged::spawn(
        std::time::Duration::from_millis(50),
        1,
        || async {
            // External call here
        },
    )
    .await
})
.await
```

* `retry::spawn` - Outer layer for retry with backoff
* `hedged::spawn` - Inner layer for parallel speculative requests
