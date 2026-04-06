# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

lifecycle-job-hook is a Kubernetes validating admission webhook that triggers Jobs based on lifecycle events of other Jobs and CronJobs. It implements a "PostComplete" hook pattern where completing Jobs or CronJobs can automatically trigger follow-up Jobs.

## Common Development Commands

- `make dev` - Deploy to local Kubernetes with Skaffold hot reload and port forwarding
- `go mod tidy` - Manage Go dependencies
- `go test ./...` - Run all tests
- `go build -o lifecycle-job-hook` - Build the binary locally

## High-Level Architecture

### Core Components

1. **Admission Webhook Server** - Validates UPDATE operations on Jobs/CronJobs using controller-runtime's webhook server on port 9443
2. **Hook Processor** - Detects completion events and creates new Jobs based on template annotations
3. **Template System** - Uses annotations to specify Job templates that should be triggered

### Hook Configuration

Jobs and CronJobs are configured via annotations:
- `lifecycle-job-hook.kaidotio.github.io/hook: PostComplete` - Enables the hook
- `lifecycle-job-hook.kaidotio.github.io/job-template-*` - Specifies the Job template to create

### Trigger Conditions

- **Jobs**: Triggered when a Job gets a completion timestamp
- **CronJobs**: Triggered when active jobs transition from >0 to 0

### Design Patterns

1. **Dynamic API Groups** - Supports variant API groups via VARIANT environment variable for multi-tenancy
2. **Owner References** - Created Jobs maintain proper ownership chains to their source Job/CronJob
3. **Unique Naming** - Generates Job names with Unix timestamp suffixes to avoid conflicts
4. **Distroless Containers** - Uses gcr.io/distroless/static for minimal security footprint
5. **Health Endpoints** - Exposes /healthz (8081) and /metrics (8080) for observability