# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

The headless-page-renderer is a Python microservice that uses Playwright to render web pages in a headless Chrome browser. It's primarily used for cache warming (pre-rendering Grafana dashboards) and collecting page load performance metrics.

## Common Development Commands

- `make` or `make dev` - Run the application with auto-reload using watchexec
- `make install` - Install Python dependencies with uv and download Chromium browser
- `uv sync --frozen` - Install Python dependencies (uses uv, not pip)
- `uv lock` - Update dependency locks

### Running the Application

```bash
# Basic usage - render a single URL
python main.py https://example.com

# Multiple URLs with custom interval
python main.py https://example.com https://example.org --interval 30

# With StatsD metrics
python main.py https://example.com --use-statsd --statsd-host localhost --statsd-port 8125
```

## High-Level Architecture

This is a single-file async Python application (`main.py`) that:

1. **Browser Automation** - Uses Playwright to control headless Chromium
2. **Async Processing** - Processes URLs concurrently using asyncio
3. **Metrics Collection** - Optional StatsD integration for latency monitoring
4. **Network Idle Detection** - Waits for network activity to settle before measuring load time

### Key Dependencies

- `playwright==1.46.0` - Browser automation
- `statsd==4.0.1` - Metrics reporting
- `requests==2.31.0` - HTTP requests (for Grafana integration)
- Python 3.11+ required

### Production Deployment

The application runs as a Kubernetes CronJob that:
- Fetches Grafana dashboard URLs dynamically
- Pre-renders dashboards every 5 minutes
- Reports performance metrics to StatsD/Pushgateway
- Uses non-root user (UID 65532) for security
