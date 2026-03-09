# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Husky is a Rust package that automatically installs git hooks by configuring Git's `core.hooksPath` to point to a `.husky` directory. The package uses a build script (`build.rs`) that runs during compilation to set up the git configuration.

## Common Development Commands

### Building
- `cargo build` - Build the package
- `cargo build --release` - Build optimized release version
- `cross build --target x86_64-unknown-linux-gnu` - Cross-compile for Linux GNU
- `cross build --target x86_64-unknown-linux-musl` - Cross-compile for Linux musl

### Testing
- `cargo test` - Run unit tests
- `cross test --target <target>` - Run tests for specific target

### Code Quality
- `cargo fmt` - Format code according to Rust standards
- `cargo clippy --fix` - Lint and auto-fix issues
- `cargo udeps --all-targets --all-features` - Check for unused dependencies

## Architecture

This is a minimal Rust library package that:
1. Uses a build script (`build.rs`) that executes during compilation
2. Checks if Git's `core.hooksPath` is already configured
3. If not configured, sets it to `.husky` directory
4. Provides automatic git hook setup for the Hippocampus monorepo

The package is part of the larger Hippocampus workspace and follows the workspace's standard build patterns using cross-compilation and the mold linker when available.