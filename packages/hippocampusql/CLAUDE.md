# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

hippocampusql is a query language parser for the Hippocampus search system. It provides a robust parser for converting search queries with boolean operators (AND, OR, NOT) into a structured Abstract Syntax Tree (AST).

## Common Development Commands

### Building and Testing
- `cargo build` - Build the library
- `cargo test` - Run all tests
- `cargo fmt` - Format code according to Rust conventions
- `cargo clippy` - Run linter for code improvements

### Development Workflow
- `cargo watch -x test` - Run tests on file changes (requires cargo-watch)
- `cargo doc --open` - Generate and view documentation

## High-Level Architecture

### Query AST Structure
The parser produces three types of query nodes:
- **Term**: Simple unquoted search terms
- **Phrase**: Quoted strings for exact phrase matching  
- **Operation**: Boolean expressions combining terms/phrases with AND, OR, NOT operators

### Parser Implementation
- Built using the `nom` parser combinator library for composable parsing
- Recursive descent parsing allows nested boolean operations
- Whitespace handling is built into the parser combinators
- Error handling provides structured parsing errors

### Query Grammar
```
query         := operation | term_or_phrase | ε
operation     := term_or_phrase operator (operation | term_or_phrase)
operator      := " AND " | " OR " | " NOT "
term_or_phrase := phrase | term
phrase        := '"' [^"]* '"'
term          := [^" ]+
```

The parser handles operator precedence through left-to-right evaluation, with operations parsed recursively to support arbitrary nesting depth.