# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Chrome DevTools Protocol Server is a containerized service that provides headless Chromium with remote debugging capabilities. It runs Chromium in a virtual display environment and exposes the Chrome DevTools Protocol for automation, testing, and web scraping purposes.

## Architecture

The service uses supervisord to manage three processes:
1. **Xvfb** - Virtual framebuffer providing display :0 at 1920x1080 resolution
2. **Chromium** - Running with remote debugging enabled on port 9222
3. **socat** - TCP proxy forwarding external port 59222 to internal port 9222 (works around Chromium issue #41487252)

## Common Development Commands

### Building
```bash
docker build -t chrome-devtools-protocol-server .
```

### Running Locally
```bash
# Run with port forwarding
docker run -p 59222:59222 chrome-devtools-protocol-server

# Run with custom name for easier management
docker run -d --name cdp-server -p 59222:59222 chrome-devtools-protocol-server
```

### Testing
```bash
# Verify the service is running
curl http://localhost:59222/json/version

# List available debugging targets
curl http://localhost:59222/json

# Create a new browser context
curl -X PUT http://localhost:59222/json/new

# Close a browser context (replace {id} with actual target ID)
curl -X DELETE http://localhost:59222/json/close/{id}
```

## Key Configuration

- **Base Image**: debian:bookworm-slim
- **User**: Non-root (UID 65532)
- **Exposed Port**: 59222 (proxied to Chromium's 9222)
- **Chromium Flags**: `--remote-debugging-port=9222 --no-sandbox --disable-gpu`

## Integration Points

This service provides Chrome DevTools Protocol access which can be used by:
- Web scraping tools
- Browser automation frameworks (Puppeteer, Playwright)
- Testing frameworks requiring browser interaction
- Screenshot/PDF generation services

## Troubleshooting

If Chromium crashes or fails to start:
1. Check container logs: `docker logs <container-id>`
2. Verify supervisord is running all processes: `docker exec <container-id> supervisorctl status`
3. Ensure sufficient memory is allocated to the container
4. Test direct connection to port 9222 inside container: `docker exec <container-id> curl localhost:9222/json`