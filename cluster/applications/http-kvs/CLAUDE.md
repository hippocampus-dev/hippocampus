# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

http-kvs is a simple HTTP-based key-value store that uses S3 (or S3-compatible storage) as its backend. It provides basic CRUD operations (GET, POST, DELETE) through HTTP endpoints.

## Common Development Commands

### Running the Development Server
- `make dev` - Runs the application with watchexec for auto-reload, configured with local MinIO credentials

### Building and Deployment
- `go build main.go` - Build the binary locally
- `docker build -t http-kvs .` - Build the Docker image

### Dependencies
- `go mod tidy` - Clean up and verify dependencies
- `go mod download` - Download dependencies

## Architecture

### Core Components
- **HTTP Server**: Listens on configurable address (default: 0.0.0.0:8080)
- **S3 Backend**: Uses AWS SDK v2 to interact with S3 or S3-compatible storage
- **Graceful Shutdown**: Implements proper signal handling with configurable grace periods

### Key Features
1. **S3 Storage**: All key-value pairs are stored as objects in an S3 bucket
2. **Content Type Detection**: Automatically detects and preserves content types
3. **Health Check**: Provides `/healthz` endpoint for Kubernetes readiness/liveness probes
4. **Configurable Options**:
   - `-address`: Server listen address
   - `-termination-grace-period-seconds`: Graceful shutdown duration
   - `-lameduck`: Period to reject new connections before shutdown
   - `-http-keepalive`: Enable/disable HTTP keep-alive

### Environment Variables
- `S3_ENDPOINT_URL`: S3 endpoint (for MinIO or other S3-compatible storage)
- `S3_BUCKET`: Target S3 bucket name
- `AWS_ACCESS_KEY_ID`: AWS/S3 access key
- `AWS_SECRET_ACCESS_KEY`: AWS/S3 secret key

### API Endpoints
- `GET /{key}`: Retrieve value for a key
- `POST /{key}`: Store value for a key (body contains the value)
- `DELETE /{key}`: Delete a key
- `GET /healthz`: Health check endpoint