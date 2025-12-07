# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a JupyterHub deployment configured for Kubernetes using KubeSpawner. It's a configuration-only deployment that uses standard JupyterHub packages without custom Python code.

## Common Development Commands

### Dependency Management
- `make all` - Lock Poetry dependencies without updating (runs `uvx poetry lock --no-update`)
- `poetry install` - Install dependencies locally (if needed for testing)
- `poetry update` - Update dependencies to latest compatible versions

### Docker Operations
- `docker build -t jupyterhub .` - Build the JupyterHub container image
- The container runs as non-root user (UID 65532) for security

## Architecture

### Components
- **JupyterHub 5.3.0** - Core hub server that manages user authentication and spawning
- **KubeSpawner 6.2.0** - Spawns user notebook pods in Kubernetes
- **Idle Culler 1.2.1** - Automatically shuts down idle user sessions to save resources
- **PycURL** - HTTP client for hub API communications

### Configuration Model
- JupyterHub configuration expected at `/usr/local/etc/jupyterhub/jupyterhub_config.py` (mounted at runtime)
- No custom authenticators or spawners - uses standard KubeSpawner
- Configuration typically includes:
  - Kubernetes namespace settings
  - User pod resource limits
  - Storage volume configurations
  - Authentication backend settings

### Deployment Pattern
1. Docker image built from this directory
2. Deployed to Kubernetes with ConfigMap containing `jupyterhub_config.py`
3. Hub spawns individual user pods using KubeSpawner
4. Idle culler monitors and cleans up unused sessions

## Development Notes
- This is a configuration-driven deployment with no custom code
- Testing happens at the deployment/integration level
- The project is transitioning from Poetry to UV for Python dependency management
- Container includes essential system libraries: libcurl, OpenSSL, SQLite3