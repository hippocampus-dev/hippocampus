# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

kube-crud is a web-based frontend application for managing Kubernetes resources. It provides a simple UI for CRUD operations on Kubernetes resources, currently focusing on CronJobs. The application is built with Preact (via CDN), uses Tailwind CSS for styling, and communicates with a backend kube-crud-server to interact with the Kubernetes API.

## Common Development Commands

### Building and Running
- `docker build -t kube-crud .` - Build the Docker image
- The application runs on port 8080 when containerized
- No build process needed - uses vanilla JavaScript with ES6 modules loaded from CDNs

### Development Workflow
Since this is a static web application without build tools:
1. Edit files directly in `components/` or modify `index.html`
2. Test changes by serving files locally or rebuilding the Docker image
3. The backend API host is configured in `constants/host.js`

## High-Level Architecture

### Application Structure
- **Frontend Framework**: Preact 10.22.1 (loaded via CDN, not bundled)
- **Styling**: Tailwind CSS (loaded via CDN)
- **Web Server**: Nginx serving static files
- **Backend Communication**: RESTful API calls to kube-crud-server

### Key Components
1. **App.js** (`components/App.js`): Main component that manages:
   - Resource selection (namespace, group, version, kind)
   - CRUD operations via API calls
   - State management for editing resources
   - URL parameter handling for deep linking

2. **Renderer Pattern**: Uses factory pattern for resource-specific rendering:
   - `DefaultRenderer`: Basic JSON display for unsupported resources
   - `BatchV1CronJobRenderer`: Specialized UI for editing CronJob environment variables
   - Extensible design - add new renderers for additional resource types

3. **API Integration**: All Kubernetes operations go through kube-crud-server:
   - Endpoint configured in `constants/host.js`
   - Uses credentials for authentication
   - Supports GET, PATCH, DELETE operations

### Important Implementation Details
- Uses Preact's `h` function for creating elements (not JSX)
- No transpilation - modern JavaScript with ES6 modules
- Real-time validation (e.g., Update button disabled when no changes)
- Confirmation dialogs for destructive operations
- Runs as non-root user (65532) in container for security