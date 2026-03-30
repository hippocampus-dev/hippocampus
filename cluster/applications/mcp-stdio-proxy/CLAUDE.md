# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

mcp-stdio-proxy is a Go-based HTTP proxy server that bridges Server-Sent Events (SSE) requests to stdio-based Model Context Protocol (MCP) servers. It acts as a transport layer, converting HTTP/SSE communication to stdin/stdout communication with MCP server processes.

## Common Development Commands

### Building
```bash
# Build the binary
go build -o mcp-stdio-proxy main.go

# Build with optimization flags (production)
go build -trimpath -o mcp-stdio-proxy -ldflags="-s -w" main.go
```

### Running
```bash
# Run directly (development)
go run main.go <mcp-server-command> [args...]

# Run with custom options
go run main.go --address=0.0.0.0:8080 --verbose <mcp-server-command>

# Example with an MCP server
./mcp-stdio-proxy --address=localhost:8080 --verbose my-mcp-server --arg1 --arg2
```

### Testing
```bash
# Run tests (if any exist)
go test ./...

# Run with verbose output
go test -v ./...
```

### Dependencies
```bash
# Update dependencies
go mod tidy

# Download dependencies
go mod download
```

### Docker
```bash
# Build Docker image
docker build -t mcp-stdio-proxy .

# Run Docker container
docker run -p 8080:8080 mcp-stdio-proxy <mcp-server-command>
```

## Architecture

### Key Components

1. **SSE Endpoint** (`GET /sse`): Establishes SSE connection and spawns MCP server process
   - Returns session endpoint URL via SSE
   - Forwards stdout from MCP server to SSE messages
   - Manages bidirectional communication

2. **Message Endpoint** (`POST /messages`): Receives MCP messages from clients
   - Routes messages to appropriate session
   - Forwards to MCP server via stdin

3. **Session Management**: Maps HTTP sessions to MCP server processes
   - Each SSE connection spawns a new MCP server process
   - Sessions are cleaned up when connections close
   - Concurrent session support via sync.Map

### MCP Protocol Support

The proxy understands the following MCP methods for logging/debugging:
- Initialize/Initialized
- Ping
- Resource operations (list, read, subscribe)
- Prompt operations (list, get)
- Tool operations (list, call)
- Progress notifications
- Various change notifications

### Command Line Flags

- `--address`: Server listen address (default: "0.0.0.0:8080")
- `--termination-grace-period-seconds`: Graceful shutdown duration (default: 10)
- `--lameduck`: Period to reject new requests before shutdown (default: 1)
- `--http-keepalive`: Enable HTTP keep-alive (default: true)
- `--verbose`: Enable verbose logging of MCP methods

### Design Patterns

1. **Process-per-session**: Each SSE connection spawns a dedicated MCP server process
2. **Channel-based communication**: Uses Go channels for request/response queuing
3. **Graceful shutdown**: Handles SIGTERM with configurable grace period
4. **Error isolation**: Process crashes don't affect other sessions