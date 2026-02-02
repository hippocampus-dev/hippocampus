# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is an AI-powered API service that provides an OpenAI-compatible chat completions endpoint with advanced agent capabilities. It uses FastAPI to serve HTTP endpoints and integrates with various AI services including OpenAI, embedding retrieval, and web search capabilities.

The service acts as an intelligent proxy that enhances OpenAI API responses with:
- Internal knowledge retrieval using embeddings
- Web search capabilities (Bing or DuckDuckGo)
- Browser automation for extracting information from web pages
- Rate limiting and telemetry

## Common Development Commands

### Development
- `make dev` - Runs the API with auto-reload using watchexec (requires OPENAI_API_KEY environment variable)
- `make install` - Installs dependencies using UV and downloads Playwright browser

### Running the Service
- `uv run -- python main.py` - Run the API service directly
- `export OPENAI_API_KEY=<YOUR_KEY>` - Required before running

### Docker
- Build: `docker build -t api .`
- The Dockerfile uses a multi-stage build with Python 3.11 and includes all necessary system dependencies

## High-Level Architecture

### Core Components

1. **main.py** - FastAPI application with endpoints:
   - `/v1/chat/completions` - OpenAI-compatible chat endpoint with optional Cortex mode
   - `/v1/messages` - Anthropic Messages API compatible endpoint using OpenAI models
   - `/v1/responses` - OpenAI Responses API endpoint (proxy mode)
   - `/v1/realtime` - WebSocket endpoint for real-time chat
   - `/v1/images/generations` - Image generation endpoint
   - `/healthz` - Health check endpoint
   - `/metrics` - Prometheus metrics endpoint

2. **api/agent/root_agent.py** - Main agent orchestrator that provides:
   - `retrieval_unknown` - Semantic search in internal knowledge base
   - `web_search` - Web search using Bing or DuckDuckGo
   - `open_url` - Extract content from URLs
   - `launch_browser_agent` - Browser automation for complex web interactions

3. **api/anthropic/** - Anthropic Messages API compatibility layer:
   - `model.py` - Pydantic models for Anthropic request/response formats
   - `adapters.py` - Transformation logic between Anthropic and OpenAI formats

4. **api/settings.py** - Configuration management using Pydantic settings with environment variables

### Key Design Patterns

1. **Agent System**: Uses a function-calling pattern where the root agent can invoke specialized sub-agents
2. **Memory Types**: Supports Redis for storing conversation context and rate limiting
3. **Telemetry**: Comprehensive OpenTelemetry integration for tracing and metrics
4. **Rate Limiting**: Token-based rate limiting per user with configurable limits
5. **Error Handling**: Retry logic for transient failures and proper error responses

### Dependencies

- **cortex**: Internal package providing LLM abstractions and agent framework
- **embedding-retrieval**: Internal service for semantic search
- **Redis**: Used for memory storage and rate limiting
- **Playwright**: Browser automation for web scraping
- **UV**: Modern Python package manager (replaces pip)

### Environment Variables

Required:
- `OPENAI_API_KEY` - OpenAI API key
- `REDIS_HOST`, `REDIS_PORT` - Redis connection
- `EMBEDDING_RETRIEVAL_URL` - URL for embedding service
- `GITHUB_TOKEN`, `SLACK_BOT_TOKEN` - For agent capabilities
- `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `GOOGLE_PRE_ISSUED_REFRESH_TOKEN` - Google OAuth

Optional:
- `CHROME_DEVTOOLS_PROTOCOL_URL` - Connect to existing Chrome instance
- `BING_SUBSCRIPTION_KEY` - Use Bing search instead of DuckDuckGo
- `SYSTEM_PROMPT` - Custom system prompt
- `MODEL` - Override default model (gpt-4o)
