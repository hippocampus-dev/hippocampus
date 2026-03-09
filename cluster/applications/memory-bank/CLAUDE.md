# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

The memory-bank service is a FastAPI-based microservice that provides vector embedding storage and retrieval capabilities for the Hippocampus platform. It uses hybrid search combining dense OpenAI embeddings with sparse BM25/BM42 embeddings stored in Qdrant vector database.

## Common Development Commands

### Primary Commands
- `make dev` - Main development command that runs the service with auto-reload via watchexec
- `make install` - Install dependencies using UV (`uv sync --frozen`)
- `make fmt` - Format code using ruff (`uvx ruff format`)
- `make lint` - Lint and auto-fix code using ruff (`uvx ruff check --fix`)
- `make all` - Run both formatting and linting

### Running the Service
- `uv run -- python main.py` - Run the service directly
- `uv run -- uvicorn main:app --host 0.0.0.0 --port 8000` - Run with uvicorn explicitly
- The service uses UV as the package manager (not pip or poetry)

## High-Level Architecture

### Core Components
- **FastAPI Application** (`main.py`): Main entry point with `/upsert`, `/query`, `/delete`, `/healthz`, and `/metrics` endpoints
- **DataStore Abstraction** (`memory_bank/datastore/__init__.py`): Abstract base class for vector store implementations with OpenAI embedding integration
- **Qdrant Implementation** (`memory_bank/datastore/qdrant.py`): Concrete implementation using Qdrant with hybrid search (dense + sparse vectors)
- **Settings** (`memory_bank/settings.py`): Pydantic settings for configuration via environment variables
- **Models** (`memory_bank/model.py`): Pydantic models for API request/response schemas
- **Categorizer** (`memory_bank/categorizer.py`): Document categorization logic
- **Merger** (`memory_bank/merger.py`): Memory merging logic for updates
- **Telemetry** (`memory_bank/telemetry.py`): OpenTelemetry integration for tracing and metrics

### Key Design Patterns
1. **Hybrid Search**: Combines dense embeddings (OpenAI Ada v2/v3) with sparse embeddings (BM25/BM42 via fastembed)
2. **Async/Await**: All I/O operations use async patterns for non-blocking performance
3. **Token-based Chunking**: Documents split using tiktoken based on configurable chunk sizes
4. **Memory Categorization**: Documents are categorized and stored with category-based UUIDs
5. **Reciprocal Rank Fusion**: Merges results from multiple search methods for better relevance
6. **OpenTelemetry Integration**: Full observability with traces exported to OTLP collector and Prometheus metrics
7. **MCP Support**: Model Context Protocol integration via fastapi-mcp mounted at `/sse`

### Service Workflow
1. **Upsert**:
   - Documents → Categorization → Chunking (tiktoken) → Generate category UUID
   - Merge with existing memories if present
   - Generate OpenAI embeddings in batches → Generate BM25/BM42 sparse embeddings
   - Store in Qdrant with both dense and sparse vectors

2. **Query**:
   - Empty queries with category filter → Retrieve all memories for category
   - Non-empty queries → Generate OpenAI embeddings → Hybrid search in Qdrant
   - Apply RRF scoring → Return top-k results

3. **Delete**: Apply metadata filter → Remove matching documents from Qdrant

### Configuration
Key environment variables:
- `OPENAI_API_KEY` - Required for embedding generation
- `OPENAI_API_TYPE` - Set to "azure" for Azure OpenAI
- `EMBEDDING_MODEL` - Options: text-embedding-ada-002, text-embedding-3-small (default), text-embedding-3-large
- `DEFAULT_CHUNK_SIZE` - Token limit for document chunks (default: 512)
- `EMBEDDING_BATCH_SIZE` - Batch size for embedding generation (default: 32)
- `QDRANT_HOST` - Qdrant server host (default: 127.0.0.1)
- `QDRANT_PORT` - Qdrant server port (default: 6333)
- `QDRANT_COLLECTION_NAME` - Collection name (default: memory-bank)
- `QDRANT_TIMEOUT` - Request timeout in seconds (default: 10)
- `QDRANT_REPLICATION_FACTOR` - Replication factor (default: 1)
- `QDRANT_SHARD_NUMBER` - Number of shards (default: 1)
- `LOG_LEVEL` - Logging level (default: info)
- `HOST` - Service host (default: 127.0.0.1)
- `PORT` - Service port (default: 8080)

### Error Handling
- **RetryableError**: Custom exception for transient failures (503 response)
- Automatic retry for OpenAI API connection errors and specific status codes (409, 429, 502, 503, 504)
- Middleware catches all exceptions and returns appropriate HTTP status codes

### Docker Deployment
- Multi-stage build with Python 3.11-slim-bookworm base
- Dependencies exported via UV to requirements.txt
- Runs as non-root user (UID 65532)
- Entry point: `python /opt/memory-bank/main.py`
