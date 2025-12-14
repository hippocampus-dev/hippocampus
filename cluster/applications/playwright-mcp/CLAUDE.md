# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

The playwright-mcp service provides browser automation capabilities through the Model Context Protocol (MCP) to the Cortex Bot. It combines Microsoft's Playwright MCP server with an HTTP/SSE proxy to enable AI agents to control web browsers for tasks like web scraping, screenshot capture, and interactive navigation.

## Architecture

This is a Docker-based service that:
1. Uses Microsoft's official Playwright MCP image (`mcr.microsoft.com/playwright/mcp:v.0.0.26`)
2. Adds the `mcp-stdio-proxy` to convert stdio-based MCP to HTTP/SSE
3. Runs Chromium in headless mode without sandbox (required for containers)
4. Deploys as a sidecar container within the cortex-bot Kubernetes pod

## Common Development Commands

### Building the Docker Image
```bash
docker build -t playwright-mcp .
```

### Testing Locally
Since this service runs as part of the cortex-bot deployment:
1. Deploy cortex-bot with Skaffold: `skaffold dev --port-forward`
2. The service will be available at `http://localhost:8000/sse` when port-forwarded

## Integration Points

- **Bot Connection**: The cortex-bot connects via `PLAYWRIGHT_MCP_URL` environment variable
- **Protocol**: Uses Server-Sent Events (SSE) for MCP communication
- **Agent**: Accessed through `PlaywrightAgent` class in the bot codebase
- **Shared Volumes**: Shares Chrome profile data with chrome-devtools-protocol-server

## Deployment

- Deployed as a sidecar container in cortex-bot, not as a standalone service
- CI/CD handled by `.github/workflows/00_playwright-mcp.yaml`
- Image pushed to `ghcr.io/hippocampus-dev/hippocampus/playwright-mcp`

## Key Configuration

The service runs with these parameters:
- `--headless`: Runs browser without GUI
- `--browser chromium`: Uses Chromium browser
- `--no-sandbox`: Required for containerized environments
- Listens on port 8000 for SSE connections