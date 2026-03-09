# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Lighthouse Exporter is a Node.js application that runs Google Lighthouse audits on specified URLs and exports the results as Prometheus metrics. It uses Puppeteer to control a headless Chrome browser and collects performance metrics for both desktop and mobile form factors.

## Common Development Commands

### Dependencies
- `npm install` - Install dependencies
- `npm ci` - Clean install dependencies (used in Docker builds)

### Running the Application
```bash
node main.mjs <urls...> [options]
```

Options:
- `--form-factors` - Form factors to test (default: ["desktop", "mobile"])
- `--audits` - Lighthouse audits to run (default: ["largest-contentful-paint", "max-potential-fid", "cumulative-layout-shift", "server-response-time"])
- `--force-tracing` - Force distributed tracing headers (default: true)
- `--interval` - Interval between runs in milliseconds (default: 60000)
- `--port` - Prometheus metrics port (default: 8080)

Example:
```bash
node main.mjs https://example.com --port 9090 --interval 30000
```

### Docker
- Build: `docker build -t lighthouse-exporter .`
- Run: `docker run -p 8080:8080 lighthouse-exporter https://example.com`

## Architecture

The application follows a continuous monitoring pattern:

1. **Main Loop** (`lighthouseLoop`): Iterates through URLs and form factors, running Lighthouse audits
2. **Mutex**: Ensures sequential execution of Lighthouse runs to prevent resource conflicts
3. **Metrics Export**: Uses OpenTelemetry to export metrics to Prometheus format on the specified port

### Key Components

- **Puppeteer Integration**: Manages Chrome browser instances for Lighthouse
- **Lighthouse Runner**: Executes performance audits with configurable parameters
- **Prometheus Exporter**: Exposes metrics in Prometheus format at `/metrics` endpoint
- **Distributed Tracing**: Optionally injects traceparent headers for correlation

### Metrics Exposed

1. `lighthouse_score` - Overall score by category (0-100)
   - Labels: category, url, form_factor
2. `lighthouse_audit` - Individual audit metrics
   - Labels: id, unit, url, form_factor
3. `lighthouse_errors_total` - Counter for Lighthouse errors
   - Labels: code, url, form_factor

### Dependencies

- `lighthouse` - Google's web performance auditing tool
- `puppeteer` - Chrome automation for Lighthouse
- `@opentelemetry/*` - Metrics collection and Prometheus export