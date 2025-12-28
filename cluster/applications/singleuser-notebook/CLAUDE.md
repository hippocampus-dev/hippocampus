# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

The singleuser-notebook is a Docker container that provides individual Jupyter notebook environments for users in a JupyterHub deployment. It extends the official Jupyter SciPy notebook with additional AI/ML and audio processing capabilities.

## Common Development Commands

### Building the Container
```bash
# Build the Docker image
docker build -t singleuser-notebook .

# Build with BuildKit (recommended for cache efficiency)
DOCKER_BUILDKIT=1 docker build -t singleuser-notebook .
```

### Testing Locally
```bash
# Run the container locally
docker run -p 8888:8888 singleuser-notebook

# Run with a mounted volume for persistent storage
docker run -p 8888:8888 -v $(pwd)/notebooks:/home/jovyan/work singleuser-notebook
```

## Architecture

This is a single-user notebook container designed to be spawned by JupyterHub's KubeSpawner in a Kubernetes environment. Key aspects:

1. **Base Image**: Built on `quay.io/jupyter/scipy-notebook:hub-5.3.0` which includes the full scientific Python stack
2. **User Context**: Runs as the `jovyan` user (UID 1000) for security
3. **Package Management**: Uses both pip and conda for installing dependencies
4. **Integration**: Includes `jupyterhub==5.3.0` for proper hub authentication and spawning

## Installed Packages

### Core Functionality
- **JupyterHub Integration**: `jupyterhub==5.3.0` - Required for hub authentication
- **Language Server**: `jupyterlab-lsp==5.0.1`, `python-lsp-server==1.9.0` - Code intelligence in JupyterLab

### AI/ML Libraries
- **OpenAI**: `openai==1.69.0` - GPT integration
- **Audio Processing**: `pydub==0.25.1`, `pyannote.audio==3.3.1` - Audio manipulation and speaker diarization

### UI Enhancement
- **Interactive Widgets**: `ipyvuetify==1.11.0` - Vue.js-based UI components
- **Async Support**: `nest-asyncio==1.6.0` - Enables nested event loops for complex async workflows

## Development Notes

- All package versions are pinned for reproducibility
- The Dockerfile uses BuildKit cache mounts to speed up pip installations
- This container is typically deployed as part of the larger Hippocampus platform's JupyterHub service
- Users' notebooks and data are typically mounted via Kubernetes PersistentVolumeClaims