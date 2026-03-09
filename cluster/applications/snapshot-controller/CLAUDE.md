# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

The snapshot-controller is a Kubernetes controller that captures website screenshots and visual diffs. It provides two Custom Resource Definitions (CRDs):

- **Snapshot**: One-time visual comparison between two URLs (baseline/target states)
- **ScheduledSnapshot**: Periodic capture of a single URL with configurable intervals

The controller uses Playwright for browser automation and supports both S3 and local file storage for captured images.

## Common Development Commands

### Build and Test
- `make all` - Generate CRD manifests using controller-gen
- `make dev` - Create Kind cluster and run Skaffold dev for hot-reload development  
- `go run main.go` - Run the controller locally (requires Kubernetes config)
- `go test ./...` - Run all tests
- `docker-compose up -d` - Start LocalStack (S3), test services, and development environment

### Code Generation
- `make manifests` - Generate CRD manifests from Go types (runs controller-gen)
- Update types in `api/v1/` then run `make manifests` to regenerate

### Deployment
- `skaffold dev` - Deploy to Kubernetes with hot-reload
- `skaffold build` - Build Docker images
- `skaffold run` - Deploy to Kubernetes (one-time)

## Architecture

### Core Components

1. **Controllers** (`internal/controllers/`)
   - `snapshot_controller.go`: Handles one-time comparisons, uses errgroup for parallel capture
   - `scheduledsnapshot_controller.go`: Manages scheduled captures with cron expressions

2. **Capture Interface** (`internal/capture/`)
   - `capture.go`: Defines the Capturer interface
   - `playwright.go`: Implementation using Playwright for browser automation
   - Supports fullPage screenshots, viewport settings, and custom headers

3. **Storage Backends** (`internal/storage/`)
   - `storage.go`: Storage interface definition
   - `s3.go`: AWS S3 implementation (works with LocalStack)
   - `file.go`: Local filesystem storage

4. **Diff Generation** (`internal/diff/`)
   - `image/`: Image comparison algorithms
     - `pixel.go`: Pixel-by-pixel comparison
     - `rectangle.go`: Draws rectangles around changed regions
   - `text/line.go`: Line-based text/HTML diff

5. **HTTP Server** (`internal/myhttp/`)
   - REST API server with router and middleware support
   - Integrated OpenTelemetry tracing and Pyroscope profiling
   - Serves Kubernetes resources via `/api/v1/` endpoints
   - Static file serving for viewer UI at `/viewer/`

### CLI Tools

1. **capture** (`bin/capture/main.go`)
   - Standalone screenshot capture without Kubernetes
   - Usage: `capture -url https://example.com -output screenshot.png`
   - Supports JPEG quality control with `-quality` flag

2. **diff** (`bin/diff/main.go`)  
   - Generate visual or text diffs between files
   - Usage: `diff -baseline file1 -target file2 -output diff.png -type pixel`
   - Supports image diff types: `pixel`, `rectangle`
   - Supports text diff type: `line`

### CRD Structure

**Snapshot CRD**:
- Compares `baseline` and `target` URLs
- Generates baseline.png, target.png, and diff images
- Supports authentication headers and viewport configuration
- Status includes capture timestamps and error details

**ScheduledSnapshot CRD**:
- Captures single URL on schedule (cron expression)
- Stores snapshots with timestamp naming
- Includes last execution time and error tracking
- Uses finalizers for proper cleanup

## Key Environment Variables

```bash
# Storage Configuration
STORAGE_TYPE=s3|file
S3_BUCKET=snapshot-storage
S3_ENDPOINT=http://localstack:4566  # For LocalStack
S3_ACCESS_KEY_ID=test
S3_SECRET_ACCESS_KEY=test
FILE_STORAGE_PATH=/tmp/snapshots

# Chrome Configuration  
CHROME_DEVTOOLS_PROTOCOL_URL=ws://localhost:9222
CHROME_PATH=/usr/bin/google-chrome

# Controller Configuration
METRICS_BIND_ADDRESS=0.0.0.0:8080
HEALTH_PROBE_BIND_ADDRESS=0.0.0.0:8081
ENABLE_LEADER_ELECTION=false

# HTTP Server Configuration
HTTP_PORT=8282
```

## Development Workflow

1. **Local Development with Kind**: Run `make dev` to start a Kind cluster with Skaffold hot-reload
2. **Testing Storage**: Use docker-compose to run LocalStack for S3 testing
3. **Testing Controllers**: Deploy test CRs from `skaffold/examples/`
4. **Debugging Captures**: Check Playwright connection and Chrome DevTools
5. **CRD Updates**: Modify types in `api/v1/`, then run `make manifests`

## Important Implementation Details

- The Snapshot controller captures baseline and target URLs in parallel using errgroup
- ScheduledSnapshot uses Kubernetes finalizers for cleanup
- All captures wait for page load before taking screenshots
- The controller requires Playwright browsers (Chromium) to be accessible
- Failed captures update the CR status with error details
- Container runs as non-root user (UID 65532) for security
- The HTTP server implements OpenTelemetry tracing for all requests
- Diff generation supports both visual (pixel/rectangle) and text (line) comparisons
- During Skaffold development, CRD group changes from `snapshot.kaidotio.github.io` to `skaffold.snapshot.kaidotio.github.io`

## Testing with Sample CRs

```yaml
# Test Snapshot
apiVersion: snapshot.kaidotio.github.io/v1
kind: Snapshot
metadata:
  name: example-diff
spec:
  baseline: https://example.com
  target: https://example.org

# Test ScheduledSnapshot  
apiVersion: snapshot.kaidotio.github.io/v1
kind: ScheduledSnapshot
metadata:
  name: hourly-capture
spec:
  url: https://example.com
  schedule: "0 * * * *"
```

## CLI Tool Examples

```bash
# Capture a screenshot
go run bin/capture/main.go -url https://example.com -output screenshot.png

# Generate pixel diff between images
go run bin/diff/main.go -baseline img1.png -target img2.png -output diff.png -type pixel

# Generate rectangle diff showing changed regions
go run bin/diff/main.go -baseline img1.png -target img2.png -output diff.png -type rectangle

# Generate HTML diff
go run bin/diff/main.go -baseline page1.html -target page2.html -output diff.html -type line
```

## Project Structure

```
snapshot-controller/
├── api/v1/                         # CRD type definitions
├── internal/
│   ├── capture/                    # Screenshot capture logic
│   ├── controllers/                # Kubernetes reconciliation logic
│   ├── diff/                       # Diff generation (image & text)
│   │   ├── image/                  # Visual diff algorithms
│   │   └── text/                   # Text/HTML diff
│   ├── myhttp/                     # HTTP server implementation
│   └── storage/                    # Storage backend implementations
├── bin/                            # CLI tools
│   ├── capture/                    # Standalone capture tool
│   └── diff/                       # Diff generation tool
├── config/                         # Kubernetes manifests (generated)
├── skaffold/                       # Skaffold configuration and examples
└── main.go                         # Controller entry point
```