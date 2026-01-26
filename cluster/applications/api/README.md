# API

<!-- TOC -->
* [API](#api)
  * [Overview](#overview)
  * [Endpoints](#endpoints)
  * [Development](#development)
<!-- TOC -->

## Overview

This is an AI-powered API service that provides OpenAI-compatible endpoints with advanced agent capabilities. It also provides an Anthropic Messages API compatible interface that uses OpenAI models as the backend.

## Endpoints

| Endpoint | Description |
|----------|-------------|
| `/v1/chat/completions` | OpenAI-compatible chat completions with optional Cortex mode for agent capabilities |
| `/v1/messages` | Anthropic Messages API compatible interface using OpenAI models |
| `/v1/responses` | OpenAI Responses API endpoint (proxy mode) |
| `/v1/realtime` | WebSocket endpoint for real-time chat |
| `/v1/images/generations` | Image generation endpoint |
| `/healthz` | Health check endpoint |
| `/metrics` | Prometheus metrics endpoint |

### Anthropic Messages API (`/v1/messages`)

The `/v1/messages` endpoint provides an Anthropic Messages API compatible interface that uses OpenAI models as the backend. This allows clients using Anthropic SDK to interact with OpenAI models.

Features:
- Uses OpenAI model names (e.g., `gpt-4o`, `gpt-4o-mini`) instead of Claude model names
- Supports streaming and non-streaming responses
- Supports tool use/function calling with Anthropic format
- Request/response format follows Anthropic Messages API specification

## Development

```sh
$ export OPENAI_API_KEY=<YOUR OPENAI API KEY>
$ make dev
```
