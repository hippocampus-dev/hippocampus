# AI/ML Services

Detailed patterns for managing AI/ML services in Docker Compose.

## Service Architecture

Each AI/ML profile typically includes three services:

| Service | Purpose |
|---------|---------|
| `{name}` | Main application (GPU-enabled) |
| `{name}-chown` | Fix volume permissions for UID 65532 |
| `{name}-downloader` | Download models from Hugging Face |

## Starting AI/ML Services

```bash
# Start with model downloads
docker compose --profile=comfyui up -d

# Start full stack (all AI services)
docker compose --profile=full up -d

# View download progress
docker compose logs -f comfyui-downloader
```

## Model Management

### Volume Locations

| Service | Volume | Container Path |
|---------|--------|----------------|
| ComfyUI | comfyui-models | /home/nonroot/ComfyUI/models |
| Stable Diffusion | stable-diffusion-models | /home/nonroot/stable-diffusion-webui/models |
| Stable Diffusion Forge | stable-diffusion-forge-models | /home/nonroot/stable-diffusion-webui-forge/models |
| llama.cpp | llama.cpp-models | /home/nonroot/llama.cpp/models |
| Yue | yue-models | /home/nonroot/yue/models |
| Ollama | ollama-models | /root/.ollama/models |

### Accessing Models via ephemeral-container

```bash
# List models
docker compose exec ephemeral-container ls /home/nonroot/ComfyUI/models

# Copy model between volumes
docker compose exec ephemeral-container cp \
  /home/nonroot/stable-diffusion-webui/models/Stable-diffusion/model.safetensors \
  /home/nonroot/ComfyUI/models/checkpoints/

# Download model with MinIO client
docker compose exec ephemeral-container mc cp \
  minio/models/custom-model.safetensors \
  /home/nonroot/ComfyUI/models/checkpoints/
```

## Common Issues

| Issue | Solution |
|-------|----------|
| Model download fails | Check `HF_HUB_TOKEN` in environment |
| Out of GPU memory | Reduce batch size or use smaller model |
| Permission denied on volumes | Run `{name}-chown` service first |
| Service exits immediately | Check logs for missing dependencies |

