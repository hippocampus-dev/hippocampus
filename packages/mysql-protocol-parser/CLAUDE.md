# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`mysql-protocol-parser` is a Rust library that provides MySQL protocol parsing capabilities without external dependencies. It implements the MySQL client/server protocol for parsing various packet types including handshake, authentication, commands, result sets, and error packets.

## Common Development Commands

- `cargo fmt` - Format code according to Rust style guidelines
- `cargo clippy --fix` - Lint code and automatically fix issues
- `cargo test` - Run all tests
- `cargo build` - Build the library
- `cargo build --release` - Build optimized release version
- `cargo doc --open` - Generate and view documentation

## High-Level Architecture

### Core Components

1. **Protocol Constants** (`constants.rs`):
   - Packet type identifiers
   - Capability flags
   - Status flags
   - Command types
   - Field types

2. **Data Types** (`types.rs`):
   - Basic protocol data structures
   - Packet headers
   - Field definitions
   - Column metadata

3. **Packet Parsers**:
   - **Header Parser** (`header.rs`): Parses MySQL packet headers
   - **Handshake Parser** (`handshake.rs`): Parses initial handshake packets
   - **Authentication Parser** (`auth.rs`): Parses authentication packets
   - **Command Parser** (`command.rs`): Parses command packets
   - **Result Set Parser** (`resultset.rs`): Parses query results
   - **Error Parser** (`error.rs`): Parses error packets

### MySQL Protocol Overview

The MySQL protocol uses a packet-based communication where each packet has:
- 3-byte packet length
- 1-byte sequence ID
- Payload data

All multi-byte integers are stored in little-endian format unless otherwise specified.
