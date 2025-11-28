# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is an IntelliJ IDEA plugin providing comprehensive IDE integration features for Kubernetes and Rails development. The plugin includes:
- Transparent support for Rails encrypted files (`.enc` files) with automatic decrypt/encrypt
- Custom Filter language support with syntax highlighting
- Jump to Kustomization action for Kubernetes manifest navigation
- Full Rails 7.0+ encryption format compatibility using AES-256-GCM

## Common Development Commands

### Build & Run
- `./gradlew build` - Build the plugin
- `./gradlew runIde` - Run IntelliJ IDEA with the plugin installed for testing
- `./gradlew buildPlugin` - Build plugin distribution (creates ZIP in `build/distributions/`)
- `./gradlew clean` - Clean build artifacts
- `make dev` - Shorthand for `./gradlew runIde`
- `make build` - Shorthand for `./gradlew buildPlugin`

### Testing
- `./gradlew test` - Run all tests
- `./gradlew test --tests "*.RailsMessageEncryptorTest"` - Run a specific test class
- `./gradlew test --tests "*.RubyMarshalTest.testDump_string_multibyte"` - Run a specific test method
- `./gradlew koverReport` - Generate code coverage report in `build/reports/kover/`
- `make test` - Shorthand for `./gradlew test`

### Plugin Verification
- `./gradlew runPluginVerifier` - Verify plugin compatibility with different IDE versions
- `./gradlew runIdeForUiTests` - Run IDE for UI testing with Robot Server plugin on port 8082

## High-Level Architecture

### Core Components

1. **Rails Encryption Support**
   - `EncryptedFileType` - Registers `.enc` extension with UTF-8 encoding
   - `EncryptedFileEditorProvider` - Transparently decrypts files to temp filesystem on open, preserves original file extensions for proper syntax highlighting
   - `EncryptedFileSaveListener` - Re-encrypts content on save using `FileDocumentSynchronizationVetoer`
   - `RailsMessageEncryptor` - AES-256-GCM encryption with Rails-compatible format (base64 with `--` separator)
   - `RubyMarshal` - Ruby Marshal protocol v4.8 implementation for payload serialization

2. **Filter Language Support**
   - `FilterLanguage` - Custom language definition
   - `FilterParser` and `FilterParserDefinition` - Language parsing infrastructure
   - `FilterSyntaxHighlighter` - Syntax highlighting implementation
   - `FilterFileType` - Associates `.filter` extension with Filter language

3. **Kubernetes Integration**
   - `JumpToKustomizationAction` - Navigate from Kubernetes manifests to their kustomization.yaml references (Ctrl+Shift+K)
   - Supports multiple kustomization files with selection dialog
   - Validates YAML structure for apiVersion/kind fields

### Key Design Patterns

1. **Temporary File Management**: Decrypted content uses IntelliJ's temp:// filesystem with automatic cleanup via `Disposer.register()`
2. **Concurrent Mapping**: Thread-safe tracking of temp-to-original file relationships using `ConcurrentHashMap`
3. **Environment Security**: Master key from `RAILS_MASTER_KEY` env var with fallback to system property
4. **Editor Policy**: Uses `FileEditorPolicy.HIDE_DEFAULT_EDITOR` to replace default editor completely
5. **Ruby Marshal Compatibility**: Implements fixnum/string types with proper length encoding for Rails interoperability

### Testing Infrastructure

- **Encryption Tests**: Round-trip encrypt/decrypt validation
- **Marshal Tests**: Coverage for zero, short, long integers and ASCII/multibyte strings
- **Edge Cases**: Handles trailing newlines, special characters, UTF-8 encoding
- **Environment Setup**: Tests use `System.setProperty()` for `RAILS_MASTER_KEY`

## Important Notes

- Requires `RAILS_MASTER_KEY` environment variable for encryption features
- Supports IntelliJ IDEA 2024.3.5+ (build 250+)
- Built with Kotlin 1.9.25 and Java 21
- Dependencies: SnakeYAML 2.2 for Kubernetes manifest parsing, JUnit for testing
- Plugin ID: `io.github.kaidotio.hippocampus`
