# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

hippocampus-web is a web frontend library for the Hippocampus search and indexing platform. It's part of the Hippocampus monorepo workspace and provides web UI components and utilities.

## Common Development Commands

### Development
- `cargo build` - Build the library
- `cargo test` - Run all tests
- `cargo test <test_name>` - Run a specific test
- `cargo fmt` - Format code
- `cargo clippy --fix` - Lint and fix issues
- `cargo doc --open` - Generate and view documentation

### Cross-compilation (if needed)
- `cross build --target x86_64-unknown-linux-gnu` - Build for Linux GNU
- `cross build --target x86_64-unknown-linux-musl` - Build for Linux musl

## Architecture

This package is currently a minimal library with basic functionality. When developing:

1. **Integration with Core**: This package will likely integrate with `hippocampus-core` for search functionality
2. **Server Communication**: Will probably communicate with `hippocampus-server` for API access
3. **Web Framework**: Consider using frameworks like Yew, Leptos, or Dioxus for Rust web development
4. **WASM Support**: May compile to WebAssembly for client-side execution

## Development Guidelines

Following the Hippocampus project conventions:
- Use full module paths (e.g., `std::env::var`) instead of abbreviated imports
- Follow the existing Rust edition 2024 standards
- Maintain compatibility with the workspace's cross-compilation targets