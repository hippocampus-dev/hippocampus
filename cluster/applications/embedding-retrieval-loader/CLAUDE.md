# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

The `embedding-retrieval-loader` is a data ingestion utility that loads Japanese Wikipedia articles from the TensorFlow Datasets wiki40b/ja dataset and uploads them to the embedding-retrieval service for vector search capabilities. It's part of the Hippocampus AI/ML infrastructure.

## Common Development Commands

### Setup and Dependencies
- `uv sync --frozen` - Install dependencies from lock file
- `uv lock` - Update dependency locks
- `uv run python -m embedding_retrieval_loader.wikipedia` - Run the Wikipedia loader

### Environment Configuration
Create a `.env` file with:
```
EMBEDDING_RETRIEVAL_URL=http://localhost:8000  # URL of the embedding service
NUMBER_OF_DOCUMENTS=1000                        # Number of Wikipedia articles to load
LOG_LEVEL=INFO                                  # Logging level (DEBUG, INFO, WARNING, ERROR)
```

## High-Level Architecture

### Key Components
1. **wikipedia.py** - Main loader module that:
   - Loads Japanese Wikipedia articles from TensorFlow Datasets
   - Parses the special wiki40b format (with START_ARTICLE, START_SECTION, START_PARAGRAPH markers)
   - Structures articles into documents with metadata
   - Uploads documents to embedding-retrieval service via async HTTP

2. **settings.py** - Configuration management using Pydantic Settings:
   - Reads from environment variables and `.env` files
   - Configures embedding service URL, document count, and logging

### Data Flow
1. TensorFlow Datasets loads wiki40b/ja articles
2. Parser extracts article structure (title, sections, paragraphs)
3. Documents are formatted with metadata (source, timestamps) and source URLs
4. Batch upsert request sent to embedding-retrieval service
5. Retry logic handles transient failures (409, 429, 502-504 errors)

### Integration Points
- **Dependency**: Links to sibling `embedding-retrieval` package for data models
- **API**: Sends UpsertRequest to `/api/documents/upsert` endpoint
- **Output**: Wikipedia articles become searchable vectors in the embedding service

### Development Notes
- Uses UV package manager (not pip)
- Requires Python 3.11+
- Async architecture with aiohttp for non-blocking HTTP
- Structured JSON logging with custom formatter
- No tests or Dockerfile currently present
