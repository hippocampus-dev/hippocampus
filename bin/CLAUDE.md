# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This `/bin` directory contains utility scripts for managing the Hippocampus monorepo. These scripts handle dependency updates, version management, GitHub repository configuration, and development workflows across multiple programming languages (Rust, Python, Go).

## Common Development Commands

### Version Management
- `./bump.sh <version>` - Update version numbers across all Cargo.toml, pyproject.toml, version.go, and tauri.conf.json files. Example: `./bump.sh 1.2.3`

### Dependency Updates
- `./cargo-update.sh` - Update all Rust dependencies (runs `cargo update` in parallel)
- `./gomod-update.sh` - Update all Go modules (runs `go mod tidy` in parallel)
- `./poetry-update.sh` - Update all Poetry lock files (uses `uvx poetry lock`)
- `./uv-update.sh [--frozen]` - Update all UV lock files for Python projects

### GitHub Integration
- `GITHUB_TOKEN=<token> ./github-local-self-hosted-runner.sh` - Run a local GitHub Actions self-hosted runner
- `./repository-settings.sh up` - Apply opinionated GitHub repository settings
- `./repository-settings.sh down` - Reset repository to default settings
- `GITHUB_TOKEN=<token> ./unregister-offline-runners.sh` - Clean up offline self-hosted runners

### Other Utilities
- `./decrypt.sh` - Decrypt all `.enc` files using armyknife tool
- `./claude-init.sh [--force] [--interval <seconds>]` - Generate CLAUDE.md documentation for subdirectories

## Architecture and Key Patterns

### Script Design Patterns
1. **Parallel Execution**: Update scripts use background processes with PID tracking for efficiency
2. **Find-based Discovery**: Scripts use `find` to locate relevant files throughout the monorepo
3. **Error Handling**: All scripts use `set -e` to fail fast on errors
4. **Environment Variables**: GitHub-related scripts require `GITHUB_TOKEN` environment variable

### Dependencies
- **armyknife**: Custom CLI tool used for Rails credentials decryption
- **uvx**: UV package executor for running Poetry commands
- **libsodium-encryptor**: Service at localhost:14701 for encrypting GitHub secrets
- **Standard tools**: jq, find, curl, etc.

### Repository Configuration
The `repository-settings.sh` script manages:
- Branch protection rules (protects main branch)
- Repository features (disables issues, projects, wiki)
- Merge strategies (only allows merge commits)
- Auto-merge and branch deletion settings
- GitHub Actions workflow permissions
- Deployment environments and secrets

## Important Notes
- Most update scripts operate recursively from the project root
- The scripts assume execution from the `/bin` directory
- GitHub API operations are hardcoded for the `kaidotio/hippocampus` repository
- Parallel execution scripts wait for all background processes to complete before exiting
