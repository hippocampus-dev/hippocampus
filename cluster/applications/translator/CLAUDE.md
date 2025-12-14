# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Slack bot application that provides automatic message translation services using OpenAI's GPT models. The bot integrates with Slack workspaces to translate messages between multiple languages (English, Japanese, Vietnamese) when enabled for specific channels.

## Common Development Commands

### Development
- `make dev` - Main development command that installs dependencies and runs the application with auto-reload using watchexec
- `make install` - Installs Python dependencies using UV package manager (equivalent to `uv sync --frozen`)

### Running the Application
- `python main.py` - Runs the application directly (requires environment variables to be set)
- The application requires these environment variables:
  - `SLACK_BOT_TOKEN` - Slack bot OAuth token
  - `SLACK_SIGNING_SECRET` - Slack app signing secret for request verification
  - `OPENAI_API_KEY` - OpenAI API key for translation services
  - Additional configuration via environment variables (see settings.py:37-71)

### Building and Deployment
- Docker build is handled at the parent repository level
- The application runs on Python 3.11 and uses UV for dependency management
- Dependencies are managed in `pyproject.toml` and locked in `uv.lock`

## High-Level Architecture

### Application Structure
- **main.py** - Application entry point containing:
  - FastAPI server setup for webhook mode
  - Socket mode handler for direct Slack connection
  - Message event handlers for translation processing
  - OpenTelemetry instrumentation for observability
  - Rate limiting via Redis
  - Brain storage for configuration persistence (S3 backend)

- **translator/** - Core module containing:
  - `settings.py` - Pydantic-based configuration management with environment variable support
  - `context_logging.py` - Contextual logging utilities
  - `telemetry.py` - OpenTelemetry setup for metrics and tracing
  - `slack/` - Slack-specific functionality:
    - `customize.py` - Channel-specific translation settings management via `/translation` command
    - `expand.py` - Handles expandable message blocks for long translations
    - `i18n.py` - Internationalization support for bot messages

### Key Design Patterns
1. **Dual Operation Modes**: Supports both webhook (via FastAPI) and socket mode connections to Slack
2. **Rate Limiting**: Redis-based sliding window rate limiter to prevent API abuse
3. **Configuration Storage**: Uses "Brain" abstraction (S3 backend) to persist channel translation settings
4. **Structured Output**: Uses OpenAI's structured output feature with Pydantic models for reliable translation formatting
5. **Async Architecture**: Fully async implementation using asyncio for handling concurrent requests
6. **Observability**: Comprehensive OpenTelemetry instrumentation for HTTP clients, Redis, and custom metrics

### Translation Workflow
1. User posts message in Slack channel
2. Bot checks if translation is enabled for the channel
3. Validates user permissions and rate limits
4. Sends message to OpenAI with specific translation rules
5. Parses structured response and posts translations
6. For long messages, creates expandable buttons instead of inline text
7. Tracks token usage and performance metrics

### External Dependencies
- **cortex** - Internal package providing LLM abstractions, rate limiting, and storage interfaces
- **OpenAI API** - Used for message translation with GPT models
- **Redis** - Used for rate limiting and caching
- **S3** - Used for persistent storage of channel configurations