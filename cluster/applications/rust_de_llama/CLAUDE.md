# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

rust_de_llama is a Rust-based HTTP server that provides a REST API for text generation using llama.cpp models. It features parallel request processing, OpenAI-compatible API endpoints, and efficient memory management through FFI bindings to llama.cpp.

## Common Development Commands

### Build & Development
- `make dev` - Run development server with auto-rebuild on file changes (uses watchexec and mold linker, enables CUDA if available)
- `make all` - Run complete build pipeline: format, lint, tidy, and build for all targets
- `make fmt` - Format code with cargo fmt
- `make lint` - Run clippy with auto-fix through the parent Makefile system
- `make tidy` - Check for unused dependencies with cargo-udeps
- `make targets` - Cross-compile for x86_64-unknown-linux-gnu target

### Testing
- `cargo test` - Run all tests
- `cargo test <test_name>` - Run specific test by name
- `cross test -q --release --target x86_64-unknown-linux-gnu` - Run tests for specific target

### Running the Server
- `cargo run --features cuda` - Run with CUDA support
- `cargo run` - Run with CPU-only support
- `cargo run -- --help` - View all command-line options
- `cargo run -- --preload-model <model_file>` - Preload a specific model on startup

### Build Features
- `default` - Enables tracing
- `tracing` - OpenTelemetry tracing support
- `jemalloc` - Use jemalloc allocator for better memory performance
- `cuda` - Enable CUDA GPU acceleration (requires CUDA toolkit)

## High-Level Architecture

### Core Components

1. **FFI Bridge (`src/lib.rs`)**
   - Uses autocxx to generate safe Rust bindings for llama.cpp C++ API
   - Manages llama.cpp backend initialization, model loading, tokenization, and sampling

2. **Model Manager (`src/model_manager.rs`)**
   - Manages multiple loaded models with async RwLock for concurrent access
   - Implements lazy loading pattern - models are loaded on first request
   - Each model gets its own ParallelProcessor instance

3. **Parallel Processor (`src/parallel.rs`)**
   - Handles concurrent request processing with configurable parallel slots
   - Each slot maintains its own sampling state and token cache
   - Implements efficient batch processing for multiple simultaneous requests
   - Uses channels for streaming token responses

4. **HTTP Server (`src/main.rs`)**
   - Axum-based async HTTP server with two endpoints:
     - Main server: health check and chat completions API
     - Monitor server: metrics and pprof profiling endpoints
   - Graceful shutdown with configurable lameduck period

5. **API Handlers (`src/handler/`)**
   - `chat_completions.rs` - OpenAI-compatible chat completions endpoint
   - Supports both streaming and non-streaming responses
   - Converts chat messages to prompts with role-based formatting

### Build System

The project uses a sophisticated build system (`build.rs`) that:
- Compiles llama.cpp as a static library
- Supports both CMake (for CUDA builds) and direct cc compilation
- Links against OpenMP for CPU parallelization
- Configures CUDA libraries when the cuda feature is enabled

### Key Design Patterns

1. **Async-First Design**: All I/O operations use Tokio async runtime
2. **Resource Pooling**: Parallel slots are pre-allocated and reused across requests
3. **Streaming Architecture**: Token generation streams results as they're produced
4. **Zero-Copy Where Possible**: Uses unsafe FFI carefully to avoid unnecessary copies
5. **Graceful Error Handling**: Custom error types with proper propagation

### Model File Organization

Models should be placed in the `models/` directory (configurable via `--model-directory`). The server expects GGUF format model files compatible with llama.cpp.

## API Endpoints

- `GET /healthz` - Health check endpoint
- `POST /v1/chat/completions` - OpenAI-compatible chat completions
- `GET /metrics` - Prometheus metrics (on monitor port)
- `GET /debug/pprof/profile` - CPU profiling endpoint (on monitor port)

## Development Tips

- Use `RUST_LOG=debug` for detailed logging during development
- The `--n-parallel` flag controls how many concurrent requests can be processed
- Adjust `--n-ctx` for model context size (default: 2048)
- Use `--preload-model` to warm up specific models on startup
- Monitor memory usage when loading multiple large models simultaneously
