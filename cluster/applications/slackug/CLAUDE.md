# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Slackug is a Slack bot application that manages user groups, allowing teams to create mentionable groups of users within Slack channels. It's built with FastAPI, uses slack-bolt for Slack integration, and supports persistent storage via S3.

## Common Development Commands

### Development Workflow
- `make install` - Install dependencies using UV package manager
- `make dev` - Run development server with auto-reload (watchexec + uvicorn)
- `uv sync --frozen` - Install exact dependencies from lock file
- `uv run -- python main.py` - Run the application directly

### Docker Commands
- `docker build -t slackug .` - Build Docker image
- Image uses multi-stage build for optimization

## High-Level Architecture

### Core Components

1. **main.py** - Application entry point that:
   - Initializes FastAPI app with Slack Bolt integration
   - Sets up event handlers for Slack interactions
   - Configures OpenTelemetry instrumentation
   - Manages both socket mode and webhook endpoints

2. **slackug/slack/manager.py** - User group management logic:
   - CRUD operations for user groups
   - Channel-scoped and global group support
   - Mention handling with `!ug` command
   - Modal/view interactions for UI

3. **slackug/brain.py** - Persistent storage abstraction:
   - Abstract `Brain` interface for storage operations
   - `S3Brain` implementation for AWS S3 backend
   - Handles user group data persistence

4. **slackug/settings.py** - Configuration management:
   - Pydantic-based settings with validation
   - Environment variable support
   - S3 and Slack configuration options

### Key Design Patterns

- **Event-driven architecture**: Responds to Slack events and commands
- **Modal-based UI**: Uses Slack modals for user interactions
- **Retry mechanism**: Built-in retry logic for transient failures
- **i18n support**: Multi-language support (EN, JA, VI)
- **Observability**: Comprehensive OpenTelemetry instrumentation

### Environment Variables

Required:
- `SLACK_BOT_TOKEN` - Slack bot user OAuth token
- `SLACK_SIGNING_SECRET` - Slack app signing secret

Optional:
- `S3_*` - S3 configuration for brain storage
- `SOCKET_MODE_TOKEN` - For socket mode connection
- `OTEL_*` - OpenTelemetry configuration

### Dependencies

The project uses UV package manager with key dependencies:
- FastAPI + Uvicorn for web framework
- slack-bolt + slack-sdk for Slack integration
- boto3 for AWS S3 storage
- OpenTelemetry for observability
- Pydantic for configuration and validation

## Important Notes

- No test suite currently exists - consider adding tests when modifying code
- Uses UV package manager (not pip) - always use `uv` commands
- Supports both socket mode and webhook endpoints
- Error handling includes retryable vs non-retryable errors
- User groups can be channel-scoped or global