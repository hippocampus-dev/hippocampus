# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

lifecycle-job-hook is a Kubernetes Validating Admission Webhook that executes user-defined jobs based on resource lifecycle events. Currently supports PostComplete hooks that trigger new jobs when existing Jobs or CronJobs complete.

## Common Development Commands

### Development
```bash
# Start development with hot-reload (requires local Kubernetes cluster)
make dev

# Build Docker image
docker build -t lifecycle-job-hook .

# Run tests (no tests currently exist)
go test ./...
```

### Dependency Management
```bash
# Update Go dependencies
go mod tidy
go mod download
```

## High-Level Architecture

### Core Components

1. **Webhook Handler** (`main.go:handler`)
   - Processes admission webhook requests for Job/CronJob UPDATE operations
   - Detects completion events and triggers new job creation
   - Location: main.go:114-283

2. **PostComplete Hook Logic**
   - **For Jobs**: Triggers when `Status.CompletionTime` changes from nil to non-nil
   - **For CronJobs**: Triggers when all active jobs complete (`Status.Active` becomes empty)
   - Creates new jobs using template specified in annotations

3. **Job Creation** (`createNewJobFromMetadata`)
   - Reads template configuration from annotations
   - Creates new Job with OwnerReference to triggering resource
   - Generates unique names using timestamp
   - Location: main.go:285-335

### Required Annotations

Resources must have these annotations to use PostComplete hooks:
- `lifecycle-job-hook.kaidotio.github.io/hook: PostComplete`
- `lifecycle-job-hook.kaidotio.github.io/job-template-group: <api-group>`
- `lifecycle-job-hook.kaidotio.github.io/job-template-version: <version>`
- `lifecycle-job-hook.kaidotio.github.io/job-template-kind: <kind>`
- `lifecycle-job-hook.kaidotio.github.io/job-template-name: <resource-name>`

### Key Design Patterns

- **Admission Webhook Pattern**: Intercepts Kubernetes API requests to inject behavior
- **Annotation-Driven Configuration**: All settings via Kubernetes annotations
- **Controller-Runtime Based**: Uses standard Kubernetes controller patterns
- **Environment Variable Configuration**: Server settings via env vars with type-safe defaults