# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

The hippocampus-client package is a Rust library that provides gRPC client functionality for the Hippocampus platform. It currently implements a basic Greeter service client as a foundation for service-to-service communication.

## Common Development Commands

### Build and Test
- `cargo build` - Build the library
- `cargo test` - Run tests
- `cargo fmt` - Format code
- `cargo clippy --fix` - Lint and fix issues
- `make dev` - Auto-rebuild on file changes (from parent directory)

### Protocol Buffer Compilation
The build.rs automatically compiles protobuf definitions from `../../proto/helloworld.proto` during the build process. No manual proto compilation is needed.

## Architecture

### gRPC Client Structure
- **Proto Definitions**: Located in `../../proto/` directory (shared across monorepo)
- **Code Generation**: Uses tonic-build to generate Rust code from proto files
- **Client Implementation**: Currently provides a simple hello_world::greeter_client module
- **Service Endpoint**: Configured to connect to `http://127.0.0.1:8080`

### Integration Points
- Part of the Hippocampus Rust workspace
- Depends on the proto definitions in the monorepo root
- Uses tonic for gRPC functionality with prost for protobuf serialization
- Designed to be used by other services in the platform for inter-service communication