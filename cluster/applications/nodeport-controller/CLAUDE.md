# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

nodeport-controller is a Kubernetes controller that automatically annotates NodePort services with the cluster nodes' IP addresses. It solves the operational challenge of service discovery for NodePort services by maintaining an up-to-date list of node IPs in service annotations.

## Common Development Commands

```bash
make dev    # Run with Skaffold for hot-reload development (includes port-forwarding)
```

## High-Level Architecture

The controller consists of three main components:

1. **NodePort Controller** (`internal/controllers/nodeport_controller.go`)
   - Reconciles NodePort services by watching Service objects
   - Maintains annotation `{apiGroup}/nodes` with comma-separated node IPs
   - API group is configurable via `VARIANT` environment variable

2. **Node Webhook Handler** (`internal/handler/node_handler.go`) 
   - Mutating admission webhook that triggers on Node create/delete
   - Immediately updates all NodePort services when cluster topology changes
   - Ensures annotations stay current without waiting for reconciliation loops

3. **Main Entry Point** (`main.go`)
   - Sets up controller-runtime manager with leader election
   - Configures webhook server on port 9443
   - Exposes metrics (8080) and health probes (8081)

The controller uses a dual approach for reliability:
- Controller reconciliation ensures eventual consistency
- Webhook provides immediate updates on node changes

Key design decisions:
- Uses controller-runtime for Kubernetes integration
- Runs with minimal privileges (non-root, read-only filesystem)
- Supports high availability with leader election
- Multi-tenant capable through configurable API groups