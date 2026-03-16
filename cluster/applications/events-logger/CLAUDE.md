# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

events-logger is a Kubernetes controller that logs all Kubernetes events to stdout in JSON format. It runs as a deployment with leader election to ensure only one instance processes events at a time.

## Common Development Commands

### Development
- `make dev` - Runs the application in development mode using Skaffold with port forwarding

### Building
- `go build -trimpath -o events-logger main.go` - Build the Go binary locally
- Docker builds are handled automatically by Skaffold during development

## Architecture

### Core Components

1. **Controller Pattern**: Implements a standard Kubernetes controller using:
   - Informers to watch for Event resources
   - Work queue for processing events  
   - Rate limiting to handle bursts
   - Leader election to ensure single active instance

2. **Leader Election**: Uses Lease resources for leader election with:
   - 60 second lease duration
   - 15 second renew deadline
   - 5 second retry period

3. **Event Processing**: 
   - Watches all Kubernetes Event resources (v1.Event)
   - Marshals events to JSON and prints to stdout
   - Uses a configurable concurrency level (default: 2)

### Deployment Configuration

- Runs with 2 replicas for high availability
- Uses leader election so only one replica processes events
- Configured with strict security context:
  - Non-root user (65532)
  - Read-only root filesystem
  - No privilege escalation
  - All capabilities dropped
- Resource limits derived from GOMAXPROCS and GOMEMLIMIT environment variables

### Key Design Decisions

1. **JSON Output**: Events are output as JSON for easy parsing by log aggregation systems
2. **In-Cluster Only**: Designed to run inside Kubernetes clusters using in-cluster config
3. **Minimal Dependencies**: Uses standard Kubernetes client-go libraries
4. **Distroless Image**: Final container uses distroless/static for minimal attack surface