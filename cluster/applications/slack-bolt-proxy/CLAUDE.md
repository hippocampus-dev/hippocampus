# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Slack Bolt Proxy is a Python service that acts as an intermediary between Slack's Socket Mode and an HTTP backend service. It receives all Slack events (messages, commands, shortcuts, actions, views, options) and forwards them to a configurable backend endpoint with proper authentication.

## Common Development Commands

- `make dev` - Run the application with auto-reload (watches for file changes using watchexec)
- `make install` - Install dependencies using UV
- `uv run -- python main.py` - Run the application directly
- `uv sync --frozen` - Install exact dependencies from lock file
- `uv lock` - Update dependency locks
- `docker build -t slack-bolt-proxy .` - Build Docker image

### Testing
Currently no automated tests are configured. Manual testing involves:
1. Set up a Slack app with Socket Mode enabled
2. Configure environment variables
3. Run `make dev` and verify event forwarding

## Architecture

### Event Flow
1. Slack sends events via Socket Mode to this proxy
2. Proxy immediately acknowledges the event to Slack (prevents Slack timeout)
3. Proxy calculates HMAC signature for authentication
4. Proxy forwards the event to `http://{HOST}:{PORT}/slack/events` with:
   - Original Slack headers (X-Slack-Request-Timestamp, X-Slack-Signature)
   - OpenTelemetry trace context headers
   - JSON body containing the original event
5. Backend service processes the event asynchronously

### Key Components
- **main.py**: Application entry point that sets up Slack Bolt app with Socket Mode, handles all event types, and forwards to backend
- **slack_bolt_proxy/settings.py**: Pydantic-based configuration management from environment variables
- **slack_bolt_proxy/telemetry.py**: OpenTelemetry instrumentation for distributed tracing and metrics
- **slack_bolt_proxy/context_logging.py**: Custom logging that includes trace/span IDs in structured JSON format
- **slack_bolt_proxy/exceptions.py**: Custom exception types (RetryableError for transient failures)

### Configuration
Environment variables (can use `.env` file in debug mode):
- `SLACK_APP_TOKEN`: Slack app-level token for Socket Mode connection (xapp-*)
- `SLACK_BOT_TOKEN`: Bot user OAuth token (xoxb-*)
- `SLACK_SIGNING_SECRET`: Secret for verifying Slack requests
- `HOST`: Backend service host (default: 127.0.0.1)
- `PORT`: Backend service port (default: 8080)
- `METRICS_PORT`: Prometheus metrics port (default: 8081)
- `LOG_LEVEL`: Logging verbosity (default: INFO)
- `DEBUG`: Enable debug mode and `.env` file loading
- `OTEL_SERVICE_NAME`: Service name for telemetry (default: slack-bolt-proxy)
- `OTEL_EXPORTER_OTLP_ENDPOINT`: OpenTelemetry collector endpoint

### Observability
- Prometheus metrics exposed on METRICS_PORT
- OpenTelemetry tracing with W3C trace context propagation
- Structured JSON logging with trace/span correlation
- Graceful shutdown on SIGTERM with connection draining

### Error Handling
- RetryableError exceptions indicate transient failures that should be retried
- All exceptions are logged with full context
- Failed event forwards return appropriate HTTP status codes to backend

### Deployment
The service is deployed to Kubernetes using Kustomize:
- Base manifests in `/opt/hippocampus/cluster/manifests/slack-bolt-proxy/base/`
- Environment overlays in `/opt/hippocampus/cluster/manifests/slack-bolt-proxy/overlays/`
- Includes PodDisruptionBudget, mTLS peer authentication, and telemetry configuration