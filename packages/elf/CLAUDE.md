# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a minimal ELF (Executable and Linkable Format) parser library written in Rust with zero external dependencies (except a local `error` crate). The library provides low-level parsing of ELF binaries supporting both 32-bit and 64-bit formats with big-endian and little-endian byte ordering.

## Common Development Commands

### Building and Testing
- `cargo build` - Build the library
- `cargo test` - Run all tests
- `cargo test -- --nocapture` - Run tests with println! output visible
- `cargo fmt` - Format code according to Rust standards
- `cargo clippy` - Run linter for code improvements

### Testing Individual Components
- `cargo test test_parse_64bit` - Test 64-bit ELF parsing
- `cargo test test_parse_32bit` - Test 32-bit ELF parsing
- `cargo test test_invalid_file` - Test error handling

## High-Level Architecture

### Core Design Principles
1. **Zero-copy parsing**: Parses directly from byte slices without intermediate allocations
2. **Enum-based architecture**: Each component has 32-bit and 64-bit variants (e.g., `Header::Header32` and `Header::Header64`)
3. **Endianness support**: Handles both little-endian (ELFDATA2LSB) and big-endian (ELFDATA2MSB) formats
4. **Type safety**: Strong typing for all ELF constants (Type, Machine, Class, etc.)

### Module Structure
- `lib.rs` - Main entry point with `parse()` function and `Elf` struct
- `elf_ident.rs` - ELF identification header (first 16 bytes)
- `elf_header.rs` - Main ELF header for 32/64-bit formats
- `section_header.rs` - Section header parsing and name resolution
- `symbol.rs` - Symbol table entry parsing (`.symtab` section only)

### Key Data Flow
1. `parse()` reads the ELF identification header to determine format
2. Based on class (32/64-bit) and data encoding (endianness), it parses the main header
3. Section headers are parsed and a string table is loaded for section names
4. Symbol table is parsed if `.symtab` section exists
5. Results are stored in HashMap structures for efficient lookup by name

### Current Limitations
- Program headers are not implemented (TODO in code)
- Only `.symtab` section is parsed for symbols
- Dynamic symbol table (`.dynsym`) is not supported
- No support for relocations or dynamic linking information

## Test Infrastructure

Test binaries in `tests/fixtures/`:
- `sample32` - 32-bit ELF binary compiled from sample.c
- `sample64` - 64-bit ELF binary compiled from sample.c  
- `invalid` - Invalid file for error testing
- `sample.c` - Source file (likely `int main() { return 0; }`)

When adding new features, ensure tests cover both 32-bit and 64-bit variants.