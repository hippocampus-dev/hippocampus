# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

CSViewer is a lightweight web-based CSV file viewer built with Preact and TailwindCSS. It provides interactive CSV browsing with fuzzy search capabilities, markdown link rendering, and URL-based state management. The application uses a zero-build architecture for frontend code, loading dependencies directly from CDNs without bundling or transpilation.

## Common Development Commands

### Docker Commands
- `docker build -t csviewer .` - Build the Docker image
- `docker run -p 8080:8080 csviewer` - Run the container locally

### Local Development
Since this is a static site with no build process for frontend code:
1. Serve files locally with any static server (e.g., `python -m http.server 8080`)
2. Open http://localhost:8080 in your browser
3. Changes to JavaScript files take effect on page reload

### Playwright Tests
- `npx playwright test` - Run all Playwright tests
- `npx playwright test --ui` - Run tests with UI mode
- `npx playwright run-test-mcp-server` - Start Playwright MCP server for test automation
- Test plans are stored in `specs/` directory
- Tests require system Google Chrome installed (configured via `channel: 'chrome'`)
- Chromium sandbox is disabled for compatibility with restricted environments

## High-Level Architecture

### Zero-Build Frontend Architecture
The frontend intentionally avoids build tools and bundlers:
- Preact loaded from CDN (https://cdn.skypack.dev/preact@10.22.1)
- TailwindCSS loaded from CDN (https://cdn.tailwindcss.com)
- ES modules used directly without transpilation

### Key Components

**Frontend Application (components/App.js)**
- Implements RFC 4180-compliant CSV parser from scratch
- Bitap algorithm for fuzzy text search with configurable edit distance
- Japanese text segmentation support via Intl.Segmenter
- URL-based state management for file selection and row highlighting
- Markdown link parsing and rendering within CSV cells
- i18n support for localized columns based on browser language settings

**Configuration (index.html)**
- Defines CSV file mappings with ElasticSearch-inspired configuration
- Specifies which columns to index and display
- Controls search behavior per column (maxDistance for fuzzy matching)

**Infrastructure**
- Nginx serves static files with pre-compression (gzip_static)
- Docker container runs as non-root user (UID 65532)
- Health check endpoint at /healthz
- Proxy configuration for Google Sheets integration at /sheetserver

### Important Implementation Details

When working with the Preact components:
- Use the `h` function to create elements (imported from Preact)
- The application is a single-page app with all logic in App.js
- State management uses React/Preact hooks (useState, useEffect, useMemo)
- CSV parsing must maintain RFC 4180 compliance
- Search indexing respects the mapping configuration in index.html
