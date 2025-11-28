# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Docker Compose Environment Overview

This directory contains the Docker Compose configuration for Hippocampus's comprehensive development environment, including AI/ML services, databases, monitoring tools, and MCP servers. The main docker-compose.yaml is located at the project root (/opt/hippocampus/).

## Common Docker Compose Commands

### Starting Services with Profiles
```bash
# Start a specific AI/ML service
docker-compose --profile=stable-diffusion-webui-forge up

# Start multiple services
docker-compose --profile=comfyui --profile=llama.cpp up

# Start in detached mode
docker-compose --profile=comfyui up -d

# View logs for a specific service
docker-compose logs -f comfyui

# Stop services
docker-compose --profile=comfyui down
```

### Available Profiles
- `stable-diffusion-webui` - Original Stable Diffusion WebUI
- `stable-diffusion-webui-forge` - Stable Diffusion WebUI Forge (improved version)
- `comfyui` - ComfyUI node-based AI image generation
- `llama.cpp` - LLaMA.cpp server for LLM inference
- `yue` - Yue server

### Working with Always-On Services
```bash
# View all running services
docker-compose ps

# Access service logs
docker-compose logs -f mysql
docker-compose logs -f prometheus

# Restart a specific service
docker-compose restart redis

# Execute commands in containers
docker-compose exec mysql mysql -u root -p
docker-compose exec armyknife bash
```

## High-Level Architecture

### Service Categories

1. **AI/ML Services** (Profile-based):
   - Each service has an accompanying downloader for automatic model downloads
   - GPU-enabled with NVIDIA device reservations
   - Persistent volume mounts for models

2. **Data Stores**:
   - MySQL, Redis, Cassandra - Traditional databases
   - InfluxDB - Time series database
   - Qdrant - Vector database
   - MinIO - S3-compatible object storage

3. **Observability Stack**:
   - Prometheus - Metrics collection
   - Grafana - Visualization
   - Jaeger - Distributed tracing
   - Pyroscope - Continuous profiling
   - OpenTelemetry Collector - Telemetry pipeline

4. **Development Tools**:
   - mitmproxy - HTTP/HTTPS proxy for debugging
   - Guacamole - Remote desktop access
   - Open WebUI - Interface for Ollama
   - Multiple MCP servers for AI integrations

### Key Design Patterns

1. **Service-Specific Directories**: Each service has its own directory under `docker-compose/` containing service-specific files (e.g., `n8n/init.sh`, `yue/entrypoint.sh`, `envoy/envoy.yaml`)
2. **Encrypted Configuration**: `.enc` files contain sensitive data (decrypted with `bin/decrypt.sh`)
3. **Registry Mirrors**: Local mirrors for Docker Hub (port 5000) and GitHub Container Registry (port 5002)
4. **Network Isolation**: Separate internal network (172.16.0.0/16) for service communication
5. **Automatic Rebuilds**: `develop.watch` configuration for hot-reload during development
6. **Health Checks**: Most services include health check configurations

## Development Workflow

### Environment Setup
```bash
# Decrypt all encrypted files
bin/decrypt.sh

# Or watch for changes and auto-decrypt
make watch-decrypt

# Set required environment variables
export GITHUB_TOKEN="your-token"
export OPENAI_API_KEY="your-key"
```

### Common Development Tasks

1. **Testing AI Models**:
   ```bash
   # Start ComfyUI with models
   docker-compose --profile=comfyui up
   # Access at http://localhost:8188
   ```

2. **Database Development**:
   ```bash
   # Connect to MySQL
   docker-compose exec mysql mysql -u root -ppassword

   # Access Redis CLI
   docker-compose exec redis redis-cli
   ```

3. **Monitoring and Debugging**:
   ```bash
   # Access Grafana at http://localhost:3000
   # Access Jaeger at http://localhost:16686
   # Access Prometheus at http://localhost:9090
   ```

4. **Using MCP Servers**:
   ```bash
   # MCP servers run automatically and expose stdio interfaces
   # Used by AI models for browser automation, GitHub access, etc.
   ```

## Important Notes

- GPU support requires NVIDIA Docker runtime
- Model downloads can be large (10GB+) - ensure sufficient disk space
- Some services require specific environment variables (check docker-compose.yaml)
- Use `armyknife` container for utility operations and testing
- Registry mirrors significantly speed up image pulls in development
