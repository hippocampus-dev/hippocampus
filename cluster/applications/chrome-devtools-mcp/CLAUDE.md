# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

The chrome-devtools-mcp service provides Chrome DevTools Protocol access through the Model Context Protocol (MCP). It enables AI agents to perform advanced browser debugging, performance analysis, and network inspection tasks.

## Architecture

This is a Docker-based service that:
1. Uses Google's official Chrome DevTools MCP (`chrome-devtools-mcp` npm package)
2. Adds the `mcp-stdio-proxy` to convert stdio-based MCP to HTTP/SSE
3. Runs Chromium in headless mode with Xvfb for virtual display
4. Deploys as a sidecar container within Kubernetes pods

## Common Development Commands

### Building the Docker Image
```bash
docker build -t chrome-devtools-mcp .
```

### Testing Locally
```bash
docker run -p 8000:8000 chrome-devtools-mcp
```

## Key Configuration

- **Base Image**: node:22-slim with Chromium
- **User**: Non-root (UID 65532)
- **MCP Transport**: stdio via mcp-stdio-proxy (HTTP/SSE on port 8000)
- **Browser**: Chromium in headless mode

## Integration Points

This service provides MCP access which can be used by:
- AI agents requiring browser debugging capabilities
- Performance analysis and profiling tools
- Network traffic inspection
- Console log monitoring

## Differences from playwright-mcp

| Feature | playwright-mcp | chrome-devtools-mcp |
|---------|----------------|---------------------|
| Focus | High-level automation | Low-level debugging |
| Network analysis | Limited | Full DevTools access |
| Performance tracing | No | Yes |
| Console logs | No | Yes |
| Browser support | Chrome/Firefox/WebKit | Chrome only |
