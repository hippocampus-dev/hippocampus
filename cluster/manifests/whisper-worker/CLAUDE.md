# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Whisper-worker is a queue-based audio transcription service that uses OpenAI's Whisper model for automatic speech recognition (ASR). It processes audio files from S3-compatible storage triggered by Redis queue events and stores transcription results back to S3.

## Common Development Commands

### Development & Build
- `make dev` - Main development command that installs dependencies and runs with auto-reload via watchexec (monitors .py files and restarts on changes)
- `make install` - Install frozen dependencies using uv
- `uv sync --frozen` - Install dependencies (used by make commands)
- `uv run -- python main.py` - Run the application directly
- `docker build -t whisper-worker .` - Build Docker image with CUDA support

### Dependency Management
- `uv lock` - Update dependency lock file
- `uv add <package>` - Add new dependency

Note: This project uses UV (not pip) for Python dependency management.

## High-Level Architecture

### Project Structure
The project is split between two locations:
- **Application code**: `/opt/hippocampus/cluster/applications/whisper-worker/`
  - `main.py` - Entry point with Redis consumer loop
  - `whisper_worker/settings.py` - Pydantic-based configuration from environment
  - Docker and dependency configs
- **Kubernetes manifests**: `/opt/hippocampus/cluster/manifests/whisper-worker/`
  - `base/` - Kustomize base with Deployment, HPA, PodDisruptionBudget
  - `overlays/dev/` - Dev environment with MinIO, Redis, Istio configs

### Queue-Based Processing Flow
1. **Redis Queue Consumer**: Blocks on Redis queue (`blpop`) waiting for S3 event notifications
2. **S3 Event Parser**: Extracts bucket and object key from MinIO/S3 event structure
3. **Audio Processor**: Downloads audio file, transcribes using faster-whisper with VAD filtering
4. **Result Storage**: Saves timestamped transcriptions as gzipped text files to S3

### Key Implementation Details
- **Redis Connection**: Implements retry logic with exponential backoff and custom connection pooling to handle auto-close issues
- **S3 Event Structure**: Parses nested event format: `{"Records":[{"s3":{"bucket":{"name":"..."}, "object":{"key":"..."}}}]}`
- **Whisper Configuration**: 
  - Supports multiple model sizes (tiny to large-v3, including distil variants)
  - Uses CTranslate2 backend via faster-whisper for performance
  - VAD (Voice Activity Detection) filter enabled by default
  - Automatic language detection with probability scores
- **Error Handling**: 
  - Gracefully continues on missing files (NoSuchKey) to handle race conditions
  - Retry logic for Redis connection failures
  - Structured logging for debugging

### Configuration (via environment variables)
- `WHISPER_MODEL`: Model size selection (default: "distil-large-v3")
- `DEVICE`: Processing device - "cpu", "cuda", or "auto"
- `COMPUTE_TYPE`: Inference precision - "float16", "int8_float16", "int8" (default: "float16")
- `REDIS_HOST`, `REDIS_PORT`, `REDIS_KEY`: Queue connection details
- `S3_ENDPOINT_URL`: Optional endpoint for MinIO/alternative S3
- `S3_BUCKET`: Target bucket name (default: "whisper-worker")
- `AWS_REGION`: AWS region (default: "ap-northeast-1")

### Output Format
Transcriptions include timestamps and language detection:
```
[0.00 -> 2.50] Hello world
[2.50 -> 5.00] This is a test

Language: en (probability: 0.95)
Top 5 language probabilities:
  en: 0.95
  es: 0.03
  ...
```

Output files are saved as: `{original_filename}.{timestamp}.txt.gz`

### Docker Image Details
- Multi-stage build with CUDA support
- Base image: `nvidia/cuda:11.8.0-cudnn8-runtime-ubuntu22.04`
- Python 3.11 with system-level audio processing dependencies
- Optimized for GPU inference when available