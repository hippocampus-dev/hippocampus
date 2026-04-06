# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

hippocampus-core is a lightweight full-text search library written in Rust. It provides indexing, searching, and ranking capabilities with pluggable storage backends and tokenizers.

## Common Development Commands

### Build and Test
- `cargo build` - Build the library
- `cargo test` - Run all tests
- `cargo bench` - Run benchmarks

### Feature-specific builds
- `cargo build --features sqlite` - Build with SQLite storage backend
- `cargo build --features cassandra` - Build with Cassandra storage backend
- `cargo build --features "sqlite,tracing"` - Build with multiple features

### Run Examples
- `cargo run --example simple` - Run basic file storage example
- `cargo run --example sqlite --features sqlite` - Run SQLite storage example
- `cargo run --example cassandra --features cassandra` - Run Cassandra storage example

## High-Level Architecture

### Core Components

1. **Indexer** (`src/indexer.rs`)
   - `DocumentIndexer` processes documents asynchronously
   - Tokenizes content and creates inverted indices
   - Supports concurrent token storage

2. **Searcher** (`src/searcher.rs`)
   - `DocumentSearcher` executes queries and returns ranked results
   - Supports boolean queries (AND, OR, NOT) and phrase queries
   - Includes fragment extraction for search result highlighting

3. **Storage Traits** (`src/storage.rs`)
   - `DocumentStorage`: Interface for document persistence
   - `TokenStorage`: Interface for inverted index persistence
   - Implementations: File, SQLite, Cassandra, GCS

4. **Tokenizer Trait** (`src/tokenizer.rs`)
   - `Tokenizer`: Interface for text tokenization
   - Implementations: Lindera (Japanese/Korean), Whitespace
   - Supports parallel processing

5. **Scorer** (`src/scorer.rs`)
   - Trait-based scoring system
   - TF-IDF implementation for relevance ranking

6. **Types** (`src/types.rs` + `src/types/`)
   - `UnionFind`: Union-Find data structure with union-by-rank and path compression
   - `TeeReader`: IO utility that writes to a writer while reading from a reader
   - `LockFreeLruCache`: Sharded lock-free LRU cache for concurrent access
   - `SkipPointer`, `SkipPointers`: Pre-computed skip pointers for efficient PostingsList intersection
     - Skip pointers built at index time every 128 postings (configurable via DEFAULT_BLOCK_SIZE)
     - `galloping_search`: Exponential search for O(log n) lookup in sorted lists
     - Used automatically when posting list size ratio >= 10:1 for adaptive intersection optimization

### Key Design Patterns

- **Trait-based architecture**: Storage, tokenizers, and scorers are pluggable via traits
- **Async-first**: Built on tokio for concurrent operations
- **Compression**: Uses Rice encoding for efficient postings list storage
- **Parallel processing**: Leverages rayon for CPU-intensive tokenization
- **Adaptive intersection**: Two-pointer O(n+m) for balanced lists, skip pointer + galloping search for skewed lists

### Optional Features

- `ipadic` (default): Japanese dictionary support
- `ko-dic`: Korean dictionary support
- `tracing`: Distributed tracing support
- `cassandra`: Cassandra storage backend
- `sqlite`: SQLite storage backend
- `wasm`: WebAssembly tokenizer plugin support (wasmtime-based)