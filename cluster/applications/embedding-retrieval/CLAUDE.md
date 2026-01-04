# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

The embedding-retrieval service is a FastAPI-based microservice that provides vector embedding storage and retrieval capabilities for the Hippocampus platform. It handles document chunking, embedding generation (both dense and sparse), and semantic search using Qdrant vector database.

## Common Development Commands

### Primary Commands
- `make dev` - Main development command that runs the service with auto-reload via watchexec
- `make install` - Install dependencies using UV (`uv sync --frozen`)

### Running the Service
- `uv run -- uvicorn main:app --host 0.0.0.0 --port 8000` - Run the service directly
- The service uses UV as the package manager (not pip or poetry)

## High-Level Architecture

### Core Components
- **FastAPI Application** (`main.py`): Provides `/upsert`, `/query`, and `/delete` endpoints
- **DataStore Abstraction** (`datastore/__init__.py`): Abstract interface allowing different vector store implementations
- **Qdrant Implementation** (`datastore/qdrant.py`): Concrete vector database using hybrid search (dense + sparse embeddings)
- **Settings Management** (`settings.py`): Pydantic-based configuration via environment variables

### Key Design Patterns
1. **Hybrid Search**: Combines OpenAI embeddings (Ada v2/v3) with BM25/BM42 sparse embeddings
2. **Async/Await**: All I/O operations are async for non-blocking performance
3. **Chunking Strategy**: Documents are split based on configurable token limits before embedding
4. **Reciprocal Rank Fusion**: Merges results from dense and sparse searches
5. **OpenTelemetry Integration**: Full tracing and metrics support with context propagation

### Service Workflow
1. **Upsert**: Document → Chunking → Dense Embeddings (OpenAI) → Sparse Embeddings (BM25/BM42) → Store in Qdrant
2. **Query**: Query Text → Generate Embeddings → Hybrid Search → RRF Scoring → Return Results
3. **Delete**: Metadata Filter → Remove from Qdrant

### Configuration
Key environment variables:
- `OPENAI_API_KEY` - Required for embedding generation
- `EMBEDDING_MODEL` - Choose between text-embedding-ada-002 or text-embedding-3-small
- `QDRANT_HOST/QDRANT_PORT` - Vector database connection
- `QDRANT_COLLECTION_NAME` - Collection for storing embeddings
- `SPARSE_EMBEDDING_MODEL` - BM25 or BM42 for hybrid search

### Integration Notes
- Part of the Hippocampus AI/ML stack alongside embedding-gateway
- Supports Model Context Protocol (MCP) via fastapi-mcp
- Full observability with OpenTelemetry (traces export to platform collector)
- Containerized with multi-stage Docker build for Kubernetes deployment