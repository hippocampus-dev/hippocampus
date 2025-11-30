# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a static file server for Tauri application releases. It uses nginx to serve release files from a container running on Kubernetes.

## Architecture

- **nginx-based**: Uses nginx 1.27.3 as the web server
- **Static file serving**: Serves files from `/usr/share/nginx/html/` (mapped from local `releases/` directory)
- **Non-root container**: Runs as user 65532 for security
- **Health endpoint**: Provides `/healthz` for Kubernetes liveness/readiness probes

## Common Development Commands

### Building the Docker Image
```bash
docker build -t tauri-releases .
```

### Running Locally
```bash
# Create releases directory if it doesn't exist
mkdir -p releases

# Run the container
docker run -p 8080:8080 -v $(pwd)/releases:/usr/share/nginx/html:ro tauri-releases
```

### Testing
- Access the server at `http://localhost:8080/`
- Check health endpoint: `curl http://localhost:8080/healthz`

## Important Notes

- The `releases/` directory must contain the Tauri application files to be served
- Files are served with `Cache-Control: max-age=0, must-revalidate, public` header
- Gzip compression is enabled, with support for pre-compressed `.gz` files
- The server listens on port 8080 (not the default 80) to run as non-root