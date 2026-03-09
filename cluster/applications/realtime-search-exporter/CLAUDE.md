# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

The realtime-search-exporter is a Python-based microservice that monitors Yahoo Japan's real-time search results for specified keywords. It uses Playwright to scrape search results and exports metrics to Prometheus about how frequently keywords appear in recent tweets.

## Common Development Commands

### Development
- `make dev` - Starts the development server with watchexec for auto-reload, monitoring for the keyword 'GITHUB'
- `make install` - Installs Python dependencies via UV and Playwright browser (Chromium)

### Running the Application
- `uv run -- python main.py <keywords>` - Run the exporter with one or more keywords
- `uv run -- python main.py --interval 30 --port 9090 <keywords>` - Run with custom interval (seconds) and metrics port

### Docker
- Build image: `docker build -t realtime-search-exporter .`
- The container runs as non-root user (UID 65532)

## Architecture

The application:
1. Uses Playwright with Chromium to navigate to Yahoo Japan's real-time search
2. Scrapes tweets containing specified keywords from the last hour (tweets marked as "秒前" or "分前")
3. Exports metrics via Prometheus on port 8080 (configurable)
4. Supports HTTP proxy configuration via `HTTP_PROXY` environment variable

### Key Components
- **main.py**: Single-file application containing all logic
- **Prometheus Integration**: Uses OpenTelemetry SDK to create observable gauges
- **Metric**: `keyword_appears_per_hour` - tracks occurrences of each keyword

### Dependencies
- `playwright` - Browser automation
- `fastapi` & `uvicorn` - Web framework (though not actively used in current implementation)
- `opentelemetry-*` - Metrics and Prometheus exporter
- `prometheus-client` - HTTP server for metrics endpoint

## Development Notes

- The application requires fonts-noto-cjk for Japanese text rendering
- Browser viewport is set to 1920x1080 for consistent scraping
- The scraper waits for `networkidle` to ensure all content is loaded
- Intervals between checks default to 60 seconds to avoid rate limiting