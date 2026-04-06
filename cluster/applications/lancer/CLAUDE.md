# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

The lancer service is a FastAPI-based MCP server providing AI memory storage with semantic vector search and graph capabilities using LanceDB and lance-graph. It handles document chunking, embedding generation, entity/relation management, and graph traversal for knowledge retrieval.

## Common Development Commands

### Primary Commands
- `make dev` - Main development command that runs the service with auto-reload via watchexec
- `make install` - Install dependencies using UV (`uv sync --frozen`)

### Running the Service
- `uv run -- python main.py` - Run the service directly
- The service uses UV as the package manager (not pip or poetry)

## High-Level Architecture

### Core Components
- **FastAPI Application** (`main.py`): Provides `/upsert`, `/query`, and `/delete` endpoints
- **DataStore Abstraction** (`lancer/datastore/__init__.py`): Abstract interface with embedding generation and chunking logic
- **LanceDB Implementation** (`lancer/datastore/lancedb.py`): Concrete implementation using LanceDB for vector storage and lance-graph for graph traversal
- **Settings Management** (`lancer/settings.py`): Pydantic-based configuration via environment variables

### Key Design Patterns
1. **Semantic Search**: Dense vector search via OpenAI embeddings stored in LanceDB
2. **Graph Capabilities**: Entity and relation management with lance-graph Cypher queries for graph traversal
3. **Entity Deduplication**: Canonical name + entity type as primary key, embedding similarity as fallback candidate generation
4. **Partial Graph Loading**: Only seed entities and candidate edges are loaded into PyArrow for lance-graph queries (not full table scans)
5. **Async/Await**: LanceDB operations run in thread pool executor with OpenTelemetry context propagation
6. **MCP Support**: Model Context Protocol integration via fastapi-mcp mounted at `/sse`

### LanceDB Tables
- **knowledge**: Text chunks with OpenAI embeddings and metadata
- **entities**: Entity nodes with canonical name, type, description, and embedding
- **relations**: Edges between entities with relation type and context
- **mentions**: Links between knowledge chunks and entities for provenance tracking

### Service Workflow
1. **Upsert**: Text → Chunking → Embeddings → LanceDB storage; Entities → Dedup → Storage; Relations → Storage
2. **Query**: Query embedding → Vector search on knowledge → Entity search → Graph expansion via lance-graph → Merged results
3. **Delete**: Filter-based deletion across knowledge, entities, relations, and mentions tables

### Configuration
Key environment variables:
- `OPENAI_API_KEY` - Required for embedding generation
- `OPENAI_BASE_URL` - OpenAI API base URL (supports proxy)
- `EMBEDDING_MODEL` - Options: text-embedding-ada-002, text-embedding-3-small (default), text-embedding-3-large
- `LANCEDB_PATH` - Path for LanceDB data directory (default: ./data/lancer)
- `ENTITY_SIMILARITY_THRESHOLD` - Cosine similarity threshold for entity deduplication (default: 0.92)
- `LOG_LEVEL` - Logging level (default: info)
- `HOST` - Service host (default: 127.0.0.1)
- `PORT` - Service port (default: 8080)
