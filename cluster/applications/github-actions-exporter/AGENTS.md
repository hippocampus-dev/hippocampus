# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

GitHub Actions Exporter is a Prometheus exporter for GitHub Actions queued jobs. It enables HPA-based autoscaling of GitHub Actions self-hosted runners by exposing the count of queued workflow runs as Prometheus metrics.

## Common Development Commands

### Development
- `make dev` - Run the server locally with auto-reload
- `make all` - Format, lint, tidy, and test

### Build and Test
- `go build -o github-actions-exporter main.go` - Build the binary
- `go test -race ./...` - Run tests
- `go run main.go --owner kaidotio` - Run locally

### Docker Build
- `docker build -t github-actions-exporter .` - Build the container image

## High-Level Architecture

### Core Components
1. **Token Providers**: Supports both PAT and GitHub App authentication
   - `PatTokenProvider`: Simple token-based authentication
   - `AppTokenProvider`: GitHub App JWT authentication with token caching
2. **GitHub Client**: Queries GitHub API for queued workflow runs on each `/metrics` request
3. **HTTP Server**: Exposes `/metrics` and `/healthz` endpoints

### Key Design Decisions
- **Dual Authentication**: Supports both PAT tokens and GitHub App credentials
- **Token Caching**: App tokens are cached using SWR (stale-while-revalidate) pattern with background refresh before expiry
- **Direct API Calls**: Calls GitHub API on every `/metrics` request without caching
- **Rate Limit Monitoring**: Exposes GitHub API rate limit as metrics

### Configuration Flags
| Flag | Environment Variable | Default | Description |
|------|---------------------|---------|-------------|
| `--address` | `ADDRESS` | `0.0.0.0:8080` | HTTP server address |
| `--owner` | `GITHUB_OWNER` | (required) | GitHub organization or user |
| `--repo` | `GITHUB_REPO` | (empty) | Repository name (empty = org-level) |
| `--github-token` | `GITHUB_TOKEN` | - | PAT token |
| `--github-token-file` | `GITHUB_TOKEN_FILE` | - | PAT token file path |
| `--github-app-client-id` | `GITHUB_APP_CLIENT_ID` | - | GitHub App Client ID |
| `--github-app-installation-id` | `GITHUB_APP_INSTALLATION_ID` | - | GitHub App Installation ID |
| `--github-app-private-key` | `GITHUB_APP_PRIVATE_KEY` | - | GitHub App Private Key |

### Metrics Exposed
```
# Primary metric (for HPA)
github_actions_runs_total{owner="kaidotio", repo="hippocampus", status="queued"} 5

# Rate limit monitoring
github_api_rate_limit_remaining{resource="core"} 4950
github_api_rate_limit_limit{resource="core"} 5000
```

### Authentication Flow
1. **PAT**: If `--github-token` or `--github-token-file` is specified, use PAT authentication
2. **GitHub App**: If all app flags are specified, generate JWT → exchange for access token
3. **Priority**: PAT takes precedence over GitHub App if both are specified

### API Endpoints Used
| Scope | Endpoint |
|-------|----------|
| Organization | `GET /orgs/{org}/actions/runs?status=queued&per_page=100` |
| Repository | `GET /repos/{owner}/{repo}/actions/runs?status=queued&per_page=100` |

### Integration with HPA
This exporter is designed to work with prometheus-adapter. The expected metric format is:
```yaml
# prometheus-adapter config.yaml
- seriesQuery: 'github_actions_runs_total'
  resources:
    overrides:
      namespace:
        resource: namespace
  name:
    matches: "^(.*)_total$"
    as: "${1}_queued"
  metricsQuery: '<<.Series>>{<<.LabelMatchers>>,status="queued"}'
```

## Usage Example

```bash
# PAT authentication
github-actions-exporter --owner kaidotio --github-token $GITHUB_TOKEN

# GitHub App authentication
github-actions-exporter \
  --owner kaidotio \
  --github-app-client-id $CLIENT_ID \
  --github-app-installation-id $INSTALLATION_ID \
  --github-app-private-key "$PRIVATE_KEY"

# Repository-level metrics
github-actions-exporter --owner kaidotio --repo hippocampus --github-token $GITHUB_TOKEN
```
