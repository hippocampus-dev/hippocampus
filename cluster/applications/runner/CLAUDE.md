# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

The runner application is a base Docker image for GitHub Actions self-hosted runners. It provides the foundation image used by the GitHub Actions Runner Controller to create and manage self-hosted runners for the Hippocampus project.

## Common Development Commands

### Building
- `docker build -t runner .` - Build the Docker image locally
- The CI/CD pipeline (`.github/workflows/00_runner.yaml`) automatically builds and pushes images on changes

### Deployment
- Deployment is managed through GitHub Actions and Kustomize
- The Runner CRD manifest at `/cluster/manifests/runner/base/` references this image
- Image updates trigger automatic PR creation for manifest updates

## High-Level Architecture

### Purpose
This is a minimal Ubuntu 22.04-based Docker image that includes:
- Essential tools: curl, jq, ca-certificates, gpg, gpg-agent, netcat
- GitHub CLI (gh) v2.71.2
- Clean package lists and minimal footprint

### Integration Flow
1. This base image is built and pushed to `ghcr.io/hippocampus-dev/hippocampus/runner`
2. The GitHub Actions Runner Controller references this image in its Runner CRD
3. The controller rebuilds the image with runner-specific configurations using Kaniko
4. Rebuilt images are deployed as Kubernetes pods to run GitHub Actions workflows

### Key Design Decisions
- **Minimal Base**: Only includes essential tools to keep the image small
- **GitHub CLI**: Pre-installed to enable workflow interactions with GitHub API
- **No Application Code**: This is purely an infrastructure image
- **GitOps Managed**: Updates flow through automated PRs to Kustomize manifests