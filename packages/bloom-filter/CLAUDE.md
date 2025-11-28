# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a thread-safe Bloom filter implementation in Rust with two main components:
- `BloomFilter`: A fixed-size probabilistic data structure using atomic operations for concurrent access
- `ScalableBloomFilter`: A dynamically growing bloom filter that creates new filters as capacity is reached

## Common Development Commands

### Testing
- `cargo test` - Run all tests
- `cargo test test_basic_operations` - Run specific test
- `cargo test --package bloom-filter` - Run tests for this package specifically
- `RUST_BACKTRACE=1 cargo test test_false_positive_rate_maintained` - Run with backtrace for debugging

### Building and Linting
- `cargo build` - Build the library
- `cargo build --release` - Build optimized version
- `cargo clippy` - Run linter
- `cargo fmt` - Format code

## Architecture

### BloomFilter
Located in `src/bloom_filter.rs`. Key characteristics:
- Thread-safe using `Arc<Vec<AtomicU64>>` for bit storage
- Uses double hashing strategy (h1 + i*h2) to generate multiple hash functions
- Automatically calculates optimal size and number of hash functions based on expected items and target false positive rate
- All operations (insert, contains) use `Ordering::Relaxed` for atomic operations since bit-level races are acceptable

### ScalableBloomFilter
Located in `src/scalable_bloom_filter.rs`. Key characteristics:
- Wraps multiple `BloomFilter` instances in an `RwLock<Vec<BloomFilter>>`
- Automatically creates new filters when capacity is reached
- Uses growth factor (default 2.0) to increase capacity exponentially
- Uses tightening ratio (default 0.9) to progressively reduce false positive rate in new filters
- Combined false positive rate calculated as: `1 - ∏(1 - fp_rate_i)` across all filters

### Design Patterns
1. **Atomic operations**: Uses `AtomicU64` for lock-free concurrent reads/writes to bit array
2. **Double hashing**: Generates k hash functions from 2 base hashes to avoid expensive rehashing
3. **Optimal parameters**: Calculates size as `-(n * ln(p)) / (ln(2)^2)` and hash functions as `(m/n) * ln(2)`
4. **Poisoned lock handling**: Always handles poisoned RwLocks with `unwrap_or_else(|poisoned| poisoned.into_inner())`

## Testing Notes
- False positive rate tests use statistical sampling (test 10k items not in the filter)
- Concurrent tests spawn 10 threads each inserting 1000 items
- Target false positive rate is typically 0.01, with tests allowing up to 0.02-0.03 actual rate
