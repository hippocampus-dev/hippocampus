# Hippocampus WASM Tokenizer Example

This is a sample tokenizer plugin that demonstrates how to create custom tokenizers for hippocampus using WebAssembly.

## What it does

This plugin implements a simple regex-based tokenizer that:
- Splits text on non-alphanumeric characters
- Filters out empty tokens
- Converts all tokens to lowercase

## Building

```bash
# Add the WASM target (one-time setup)
rustup target add wasm32-wasip1

# Build the WASM module
cargo build -p hippocampus-wasm-tokenizer-example --target wasm32-wasip1 --release

# Install wasm-tools if not present
command -v wasm-tools || cargo install wasm-tools

# Convert to component model
wasm-tools component new \
    target/wasm32-wasip1/release/hippocampus_wasm_tokenizer_example.wasm \
    -o /tmp/regex_tokenizer.wasm

# Validate
wasm-tools validate /tmp/regex_tokenizer.wasm
```

## Using in hippocampus

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

## Plugin Interface

All tokenizer plugins must implement the WIT interface defined in
`/opt/hippocampus/packages/hippocampus-core/wit/tokenizer.wit`:

```wit
world tokenizer {
  export tokenize: func(content: string) -> list<string>;
}
```

## Supported Languages

You can write plugins in any language that supports the WASM component model:
- Rust (using wit-bindgen)
- Go (using TinyGo with wasm-tools)
- C/C++ (using wasm-tools)
- Python (using componentize-py)

## Development Tips

1. Keep plugins stateless - each call to tokenize should be independent
2. Minimize allocations for better performance
3. Test your plugin with `wasm-tools validate` before using
4. Use `wasm-tools print` to inspect the generated component
