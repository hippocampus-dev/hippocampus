# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a minimal JSON Web Token (JWT) library that implements RSA-based signing algorithms (RS256, RS384, RS512). The library provides core functionality for signing and verifying JWTs using RSA public/private key pairs.

## Common Development Commands

- `cargo test` - Run all tests including signature verification with test RSA keys
- `cargo fmt` - Format code (follows project convention)
- `cargo clippy --fix` - Lint and auto-fix issues
- `cargo build` - Build the library
- `cargo build --features derive` - Build with jwt_derive macros enabled

## High-Level Architecture

### Core API Functions
- `sign_with_rsa(header, claims, private_key)` - Creates a signed JWT using an RSA private key
- `verify_with_rsa(jwt, public_key)` - Verifies a JWT signature and returns the claims

### Key Design Patterns
1. **No use statements**: The codebase follows a strict convention of using full module paths (e.g., `std::collections::HashMap` instead of importing HashMap)
2. **Trait-based encoding**: Generic `Encode` trait provides consistent base64 URL-safe encoding
3. **Algorithm-specific dispatching**: Match statements handle different RSA signature variants
4. **Test fixtures**: RSA keys for testing are stored in `tests/fixtures/`

### Project Structure
- `src/lib.rs` - Main library implementation with all JWT logic
- `tests/main.rs` - Integration tests for signing and verification
- `tests/fixtures/` - PEM-encoded RSA test keys

### Current Limitations
Only RSA algorithms (RS256/384/512) are implemented. The following are not yet supported:
- HMAC algorithms (HS256/384/512)
- ECDSA algorithms (ES256/384)
- RSA-PSS algorithms (PS256/384/512)
- JWT validation beyond signature verification (expiration, not-before, etc.)

## Important Notes
- This is part of the larger Hippocampus workspace
- Depends on the internal `error` package for error handling
- Uses Rust 2024 edition
- Optional `derive` feature enables jwt_derive macros