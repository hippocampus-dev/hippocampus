# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is an enterprise-grade Slack bot powered by OpenAI that provides AI-powered conversational assistance. The bot features multi-modal AI capabilities (text, images, audio, files), agent-based architecture, and comprehensive enterprise features including rate limiting, telemetry, and multi-tenant support.

## Common Development Commands

### Development and Testing
```bash
# Primary development command (installs dependencies and runs with auto-reload)
make dev

# Install dependencies only
make install

# Run directly without auto-reload
uv run -- python main.py

# Test utilities
uv run -- python slack_message_event.py --url <Slack URL>         # Emulate Slack events
uv run -- python slack_bulk_message.py --channel <ID> --messages "msg1" "msg2"
uv run -- python slack_raise_rate_limit.py --url <Slack URL>      # Test rate limiting
```

### Environment Setup
Required environment variables:
```bash
export SLACK_BOT_TOKEN=<YOUR SLACK BOT TOKEN>
export SLACK_SIGNING_SECRET=<YOUR SLACK SIGNING SECRET>
export OPENAI_API_KEY=<YOUR OPENAI API KEY>
```

## High-Level Architecture

### Agent-Based System
The bot uses a root agent (`bot/agent/root_agent.py`) that orchestrates specialized sub-agents:
- **Audio Agent**: Audio transcription and processing
- **Browser Agent**: Web browsing and search capabilities
- **Image Agent**: Image generation and analysis
- **URL Open Agent**: Direct URL content fetching
- **Grafana Agent**: Grafana dashboard queries

### Core Components
```
bot/
├── agent/
│   └── root_agent.py      # Main orchestrator for all sub-agents
├── slack/
│   ├── __init__.py        # Slack utilities and markdown renderer
│   ├── context_manager.py # Reusable conversation contexts
│   ├── customize.py       # Per-channel avatar/persona customization
│   ├── i18n.py           # Internationalization support
│   └── reporter.py        # Usage tracking and reporting
├── settings.py            # Pydantic-based configuration
├── context_logging.py     # Structured logging utilities
└── telemetry.py          # OpenTelemetry metrics and tracing
```

### Key Architectural Patterns

1. **Async-First**: Fully asynchronous implementation using asyncio for high concurrency
2. **Event-Driven**: Responds to Slack events via Socket Mode or webhooks
3. **Streaming Responses**: Supports real-time streaming of AI responses
4. **Multi-Modal Processing**: Handles text, images, audio, and files through specialized agents
5. **Observability**: Comprehensive OpenTelemetry instrumentation for all external calls

### External Dependencies

- **State Management**: Redis for caching and ephemeral data
- **Storage**: S3 for persistent file storage and brain/context management
- **AI Services**: OpenAI API for language models, embeddings, and image generation
- **Web Automation**: Playwright for browser-based operations
- **Search**: DuckDuckGo for web search capabilities

### Configuration Management

The bot uses Pydantic settings with environment variable support. Key configuration areas:
- **Slack Configuration**: Bot tokens, signing secrets, allowed teams/channels
- **AI Models**: Configurable OpenAI models for completion, embedding, image, and audio
- **Rate Limiting**: Per-user/interval limits via Redis
- **Feature Flags**: Streaming, external shared channels, restricted users

### Enterprise Features

1. **Multi-Tenant Support**: Restrict access by teams, channels, or email domains
2. **Rate Limiting**: Redis-based rate limiting per user and time interval
3. **Usage Tracking**: Comprehensive usage reporting with cost calculation
4. **Telemetry**: Full observability stack with metrics, traces, and structured logging
5. **Context Persistence**: Save and reuse conversation contexts across sessions
6. **Consistent Hashing**: Multiple Slack API tokens are distributed using consistent hashing for improved cache efficiency and rate limit management

### Load Balancing Strategy

The bot supports multiple Slack API tokens for improved throughput and reliability. Token selection uses **consistent hashing** (`ConsistentHashFactory` from cortex package) to ensure:

- **Cache Efficiency**: Same channel always routes to the same token, maximizing `conversations_join_cache` hit rate
- **Rate Limit Optimization**: Channel-level token affinity makes rate limits more predictable
- **Graceful Scaling**: Adding/removing tokens only affects ~1/n of channels, preserving existing cache entries

This is implemented at `/opt/hippocampus/cluster/applications/bot/main.py:390` using channel ID as the hash key.