# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

jwt_derive is a procedural macro crate that provides derive macros for the jwt crate in the Hippocampus workspace. It currently implements a single derive macro `#[derive(Encode)]` that generates JWT encoding implementations for structs.

## Common Development Commands

As part of the Hippocampus workspace, this crate follows the workspace conventions:

- `cargo fmt` - Format code
- `cargo clippy --fix` - Lint and fix issues  
- `cargo test` - Run tests
- `cargo build` - Build the crate
- `cargo udeps --all-targets --all-features` - Check for unused dependencies
- `cross build --target x86_64-unknown-linux-gnu` - Cross-compile for Linux

From the root workspace:
- `make fmt` - Format all workspace code
- `make lint` - Lint all workspace code
- `make test` - Run all workspace tests

## Architecture

This is a proc-macro crate that:
- Depends on `syn` for parsing Rust syntax and `quote` for generating code
- Provides the `Encode` derive macro that implements `jwt::Encode` trait for structs
- Works in conjunction with the `jwt` crate which provides the actual JWT implementation

The macro generates a simple trait implementation without any custom behavior, relying on the default implementation in the jwt crate.