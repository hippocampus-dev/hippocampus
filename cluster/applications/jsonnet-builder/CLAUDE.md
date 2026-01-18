# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

The jsonnet-builder is a Docker container that provides a consistent environment for compiling Jsonnet files into JSON. It's primarily used in the Hippocampus platform for generating Grafana dashboards and other configuration files. The container includes both the Jsonnet compiler and jsonnet-bundler for dependency management.

## Development Commands

### Building the Docker Image
```bash
docker build -t jsonnet-builder .
```

### Running the Container
```bash
# Compile a Jsonnet file
docker run \
  --mount type=bind,src=$(pwd),dst=/var/jsonnet-builder \
  --mount type=volume,src=jsonnet-bundler,dst=/var/jsonnet-bundler \
  jsonnet-builder \
  path/to/file.jsonnet

# Install dependencies only
docker run \
  --mount type=bind,src=$(pwd),dst=/var/jsonnet-builder \
  --mount type=volume,src=jsonnet-bundler,dst=/var/jsonnet-bundler \
  jsonnet-builder \
  --only-install
```

## Architecture

The container consists of two main components:

1. **Dockerfile**: Sets up the runtime environment with:
   - Bitnami Jsonnet 0.20.0 as the base image
   - Jsonnet-bundler (jb) v0.5.1 for dependency management
   - Git and CA certificates for fetching remote dependencies
   - Non-root user (UID 1001) for security

2. **entrypoint.sh**: Implements intelligent dependency caching:
   - Calculates SHA256 hash of jsonnetfile.lock.json
   - Only runs `jb install` when dependencies change
   - Supports `--only-install` flag for dependency-only operations
   - Passes all other arguments to the jsonnet command with `-J .` for library-style imports

The caching mechanism prevents unnecessary dependency reinstallation by tracking the last installed state via a hash file in the JB_HOME directory.
