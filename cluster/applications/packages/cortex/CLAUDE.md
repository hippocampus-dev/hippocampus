# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Cortex is a Python package that provides an AI agent framework built on OpenAI's API. It implements various agent types (browser, audio, image, MCP, URL, Grafana) with features like memory management, rate limiting, function caching, and intelligent summarization. The package is part of the larger Hippocampus monorepo and integrates with its Kubernetes-based architecture.

## Common Development Commands

```bash
make install    # Install dependencies using UV
make test       # Run unit tests (default make target)
make dev        # Alias for test

# Run specific tests
uv run -- python -m unittest tests.test_factory
uv run -- python -m unittest tests.test_throttled_function

# Run all agent tests
uv run -- python -m unittest discover -s tests/agent
```

## High-Level Architecture

### Core Components

1. **Agent Framework** (`cortex/llm/openai/agent/`)
   - Base `Agent` class implements chat completion loops with OpenAI function calling
   - Agents maintain context (budget, token usage, conversation history) throughout interactions
   - Function results are cached with similarity-based retrieval to avoid redundant calls
   - Each agent type specializes in specific domains (web automation, audio, images, etc.)

2. **Memory System** (`cortex/llm/openai/agent/memory.py`)
   - Redis-based vector storage using embeddings for semantic search
   - Automatically summarizes conversations when approaching token limits
   - Caches function results with expiration and similarity matching
   - Uses custom redis-om fork to fix vector field issues

3. **Model Management** (`cortex/llm/openai/model.py`)
   - Comprehensive enum definitions for all OpenAI models including pricing
   - Feature flags for each model (reasoning, tools, streaming, vision support)
   - Handles both OpenAI and Azure OpenAI endpoints transparently

4. **Resource Management**
   - **Rate Limiting** (`cortex/rate_limit.py`): Fixed/sliding window limiters with Redis backend
   - **Throttling** (`cortex/throttled_function.py`): Control function execution frequency
   - **Load Balancing** (`cortex/factory.py`): Round-robin and consistent hashing distribution across resources
     - `RoundRobinFactory`: Simple round-robin distribution
     - `ConsistentHashFactory`: Consistent hashing with virtual nodes for key-based affinity
     - `ConsistentHashRing`: Low-level consistent hashing ring implementation with configurable virtual nodes

5. **Storage & Utilities**
   - **Brain** (`cortex/brain.py`): S3-based persistent storage abstraction
   - **URL Shortener** (`cortex/__init__.py`): Async/sync URL shortening with retry logic
   - **Custom Exceptions** (`cortex/exceptions.py`): Differentiate retryable vs permanent failures

### Agent Implementation Pattern

Each agent follows this pattern:
1. Inherit from base `Agent` class
2. Define specialized functions with `@function_metadata` decorator
3. Functions specify caching policy, dependencies, and budget requirements
4. Agent manages conversation flow and function execution within budget constraints

### Key Integration Points

- **Environment Variables**:
  - `OPENAI_API_TYPE=azure`: Use Azure OpenAI endpoints
  - `HTTP_PROXY`/`HTTPS_PROXY`: Proxy configuration
  - `SSL_CERT_FILE`: Custom SSL certificates
  - Redis connection configured via standard Redis environment variables

- **Dependencies**:
  - Uses UV package manager (not pip) with locked dependencies
  - Custom redis-om fork for vector similarity search
  - Requires Python 3.11+

### Testing Strategy

- Unit tests use Python's built-in `unittest` framework
- Async tests use `IsolatedAsyncioTestCase`
- Tests focus on core utilities (factory, throttling) and agent functionality
- Integration with larger Hippocampus test infrastructure when deployed