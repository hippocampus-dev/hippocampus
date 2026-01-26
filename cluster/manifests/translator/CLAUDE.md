# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Common Development Commands

### Running the Translator Service
- `make dev` - Run the service in development mode with hot-reload via watchexec
- `make install` - Install dependencies using UV (run this before development)

### Environment Variables Required
```sh
export SLACK_BOT_TOKEN=<YOUR SLACK BOT TOKEN>
export SLACK_SIGNING_SECRET=<YOUR SLACK SIGNING SECRET>
export OPENAI_API_KEY=<YOUR OPENAI API KEY>
export SLACK_BOT_MEMBER_ID=<YOUR SLACK BOT MEMBER ID>
export REDIS_HOST=<REDIS HOST>
export REDIS_PORT=<REDIS PORT>
export URL_SHORTENER_URL=<URL SHORTENER SERVICE>
```

## High-Level Architecture

### Service Overview
The Translator service is a Slack bot that provides real-time translation capabilities using OpenAI's GPT models. It integrates with Slack via both HTTP endpoints and Socket Mode, automatically translating messages in configured channels.

### Key Components

1. **Main Application (`main.py`)**
   - FastAPI-based HTTP server with Slack event handling
   - Socket Mode support for real-time Slack events
   - OpenTelemetry instrumentation for distributed tracing
   - Prometheus metrics for monitoring

2. **Settings Management (`translator/settings.py`)**
   - Pydantic-based configuration with environment variable support
   - Configurable rate limiting, model selection, and access controls
   - Support for Redis-based rate limiting and S3-based brain storage

3. **Slack Integration (`translator/slack/`)**
   - Custom translation configuration per channel
   - Message expansion for long translations
   - Internationalization support
   - Markdown rendering with URL shortening

### Architecture Patterns

1. **Rate Limiting**
   - Redis-based sliding window rate limiter
   - Configurable per-channel limits
   - Token-based usage tracking

2. **Brain Storage**
   - S3-based persistent storage for translation configurations
   - Channel-specific translation settings
   - Message expansion data storage

3. **Error Handling**
   - Retryable errors for transient failures
   - User-friendly error messages with i18n support
   - Comprehensive error tracking via OpenTelemetry

4. **Access Control**
   - Team-based restrictions
   - Channel allowlisting
   - Email domain filtering
   - Restricted user handling

### Translation Workflow
1. Message received from Slack
2. User and channel validation
3. Rate limit check
4. Translation settings loaded from brain storage
5. OpenAI API call with structured output
6. Response formatting with optional collapse/expand
7. Metrics and tracing data collection

### Kubernetes Deployment
- Base deployment with security context and resource limits
- Kustomize overlays for different environments
- Integration with Istio service mesh
- Horizontal Pod Autoscaler configuration
- MinIO and Redis as dependencies