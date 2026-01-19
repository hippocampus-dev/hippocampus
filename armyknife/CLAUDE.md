# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Armyknife is a CLI utility tool written in Go that provides various development and operational utilities. It follows a Swiss Army knife design pattern with multiple subcommands for different tasks.

## Common Development Commands

### Building
```bash
# Standard build (without sqlite-vec support)
go build -o armyknife main.go

# Build with sqlite-vec support (requires CGO)
CGO_ENABLED=1 go build -o armyknife main.go

# Cross-platform builds (without sqlite-vec support)
GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -o armyknife main.go
```

**Note**: The `searchx` command requires CGO to be enabled for sqlite-vec support. When CGO is disabled, the command will return an error message indicating that CGO_ENABLED=1 is required.

### Testing
```bash
go test ./...
go test -v ./internal/rails_message_encryptor/
go test -v ./internal/marshal/ruby/
```

### Running
```bash
./armyknife <command> [options]
./armyknife --help  # See all available commands
```

### Dependencies
```bash
go mod tidy
go mod download
```

## High-Level Architecture

### Command Structure
The project follows a consistent pattern for implementing commands:

1. **Command Definition**: `/cmd/<command>.go` - Defines the Cobra command
2. **Arguments**: `/pkg/<command>/args.go` - Struct definitions for command arguments with validation tags
3. **Implementation**: `/pkg/<command>/<command>.go` - Core business logic

### Available Commands
- `bakery` - Cookie authentication service client
- `completion` - Shell completion generation for bash/zsh/fish
- `echo` - Echo server functionality
- `egosearch` - Slack message search with fuzzy finding
- `grpc` - gRPC utilities with call/catch subcommands
- `llm` - LLM integration (fillcsv for CSV processing via OpenAI)
- `mcp-notify` - MCP server for desktop notifications
- `proxy` - TCP/HTTP reverse proxy implementation
- `rails` - Rails credentials management (show/edit/diff)
- `s3` - S3 object viewer with fuzzy search
- `searchx` - Vector search functionality using SQLite-vec (requires CGO_ENABLED=1)
- `selfupdate` - Auto-update functionality via GitHub releases
- `serve` - Static file server

### Key Design Patterns

1. **Modular Command Registration**: Commands are registered in `/cmd/root.go` using a clean pattern where each command package exports a `GetRootCmd()` function.

2. **Validation**: Uses `github.com/go-playground/validator/v10` with struct tags for input validation.

3. **Error Handling**: Consistent use of `golang.org/x/xerrors` for wrapped errors with context.

4. **Protocol Buffers**: Uses protobuf definitions in `/armyknifepb/` for structured data exchange.

5. **External Service Integration**: Integrates with multiple services (AWS S3, Slack, OpenAI, GitHub) using their respective SDK clients.

### Package Organization
- `/cmd/` - Command-line interface definitions using Cobra
- `/pkg/` - Public packages containing business logic for each command
- `/internal/` - Internal packages for shared utilities (Rails encryption, Ruby marshaling, OpenAI client)
- `/armyknifepb/` - Protocol buffer definitions and generated code