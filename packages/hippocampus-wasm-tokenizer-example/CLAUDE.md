# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a WebAssembly-based tokenizer plugin for the hippocampus search/indexing system. It demonstrates how to create custom tokenizers using the WASM component model and WIT (WebAssembly Interface Types).

## Common Development Commands

### Building the WASM Module

```bash
# One-time setup: Add WASM target
rustup target add wasm32-wasip1

# Build the WASM module
cargo build -p hippocampus-wasm-tokenizer-example --target wasm32-wasip1 --release

# Install wasm-tools (if not present)
cargo install wasm-tools

# Convert to component model
wasm-tools component new \
    target/wasm32-wasip1/release/hippocampus_wasm_tokenizer_example.wasm \
    -o /tmp/regex_tokenizer.wasm

# Validate the component
wasm-tools validate /tmp/regex_tokenizer.wasm
```

### Testing and Inspection

```bash
# Inspect the generated component
wasm-tools print /tmp/regex_tokenizer.wasm
```

## High-Level Architecture

### WIT Interface Contract

This plugin implements the `tokenizer.wit` interface defined at `/opt/hippocampus/packages/hippocampus-core/wit/tokenizer.wit`:

```wit
world tokenizer {
  export tokenize: func(content: string) -> list<string>;
}
```

All tokenizer plugins must export a `tokenize` function that takes a string and returns a list of tokens.

### Component Model

- **lib.rs** - Uses `wit-bindgen` to generate Rust bindings from the WIT definition
- **wit/world.wit** - Local WIT interface definition (package: `hippocampus:tokenizer@0.1.0`)
- **Cargo.toml** - Configured as a `cdylib` library with size-optimized release profile

### Integration Pattern

Tokenizer plugins are loaded into hippocampus-core via the `WasmTokenizer` type:

```rust
use hippocampus_core::tokenizer::wasm::WasmTokenizer;

let tokenizer = WasmTokenizer::from_file("/path/to/regex_tokenizer.wasm")?;
let indexer = hippocampus_core::indexer::DocumentIndexer::new(
    document_storage,
    token_storage,
    tokenizer,
    schema,
);
```

## Important Design Constraints

1. **Stateless Plugins** - Each call to `tokenize()` must be independent with no shared state
2. **Size Optimization** - Release profile uses `opt-level = "s"`, LTO, and stripping for minimal WASM size
3. **WASI Preview 1** - Target is `wasm32-wasip1`, not `wasm32-unknown-unknown`
4. **Component Model Required** - Raw WASM modules must be converted using `wasm-tools component new`

## Multi-Language Support

The WIT interface can be implemented in any language supporting WASM components:
- **Rust** - wit-bindgen (current implementation)
- **Go** - TinyGo with wasm-tools
- **C/C++** - wasm-tools
- **Python** - componentize-py
