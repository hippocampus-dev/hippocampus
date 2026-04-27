# Rust Testing Patterns

How to write consistent tests in Rust crates.

## Unit vs Integration Tests

| Test Kind | Location | Example |
|-----------|----------|---------|
| Unit | Inline `#[cfg(test)] mod tests` next to code under test | `packages/hippocampus-core/src/types/lru.rs` |
| Integration | `tests/main.rs` in the crate root | `packages/elf/tests/main.rs` |

## Unit Test Structure

```rust
#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_basic_operations() {
        // test logic
    }
}
```

## Integration Test Structure

```rust
#[test]
fn ok() -> Result<(), error::Error> {
    let mut file = std::fs::File::open("tests/fixtures/sample64")?;
    // test logic
    Ok(())
}
```

| Practice | Reason |
|----------|--------|
| Return `Result<(), error::Error>` when using `?` | Propagate fallible I/O through the repo error type |
| Short function names (`ok`, `err`, `invalid`) | Integration tests describe scenarios, not units |
| `if let Some(x) = ... { ... } else { panic!("<what is missing>") }` | Fail fast with explicit message when destructuring fails |

## Fixtures

Store binary or text fixtures under `tests/fixtures/` in the crate root.

| Load Method | When |
|-------------|------|
| `include_str!("fixtures/<name>")` | Small text fixtures embedded at compile time |
| `std::fs::File::open("tests/fixtures/<name>")` | Binary fixtures or large files |
