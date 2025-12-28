# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is the Kubernetes manifest configuration for the Hippocampus platform's Fluentd log collection and aggregation system. It consists of three main components:

1. **Fluentd Forward (DaemonSet)** - Collects logs from each Kubernetes node
2. **Fluentd Aggregator (StatefulSet)** - Centralizes and processes logs
3. **Nginx Proxy (Deployment)** - Load balances traffic to aggregator instances

The manifests use Kustomize for environment-specific configurations with a base configuration and dev overlay.

## Common Development Commands

### Deploying to Kubernetes
```bash
# Deploy to development environment
kubectl apply -k overlays/dev/

# Deploy base configuration only
kubectl apply -k base/

# Preview generated manifests without applying
kubectl kustomize overlays/dev/
```

### Building Custom Fluentd Aggregator Image
```bash
# The aggregator uses a custom image built from /opt/hippocampus/cluster/applications/fluentd-aggregator/
cd /opt/hippocampus/cluster/applications/fluentd-aggregator
docker build -t ghcr.io/hippocampus-dev/hippocampus/fluentd-aggregator .
```

### Viewing Logs
```bash
# Check fluentd-forward logs on a specific node
kubectl logs -n fluentd -l app=fluentd-forward --tail=100

# Check aggregator logs
kubectl logs -n fluentd -l app=fluentd-aggregator --tail=100

# Check proxy logs
kubectl logs -n fluentd -l app=fluentd-aggregator-proxy --tail=100
```

## High-Level Architecture

### Directory Structure
- **base/** - Core Kubernetes resources and configurations
  - DaemonSet for log collection (fluentd-forward)
  - StatefulSet for log aggregation (fluentd-aggregator)
  - Deployment for nginx proxy
  - ConfigMaps for each component's configuration
  - RBAC resources (ServiceAccount, ClusterRole, ClusterRoleBinding)
  
- **overlays/dev/** - Development environment specific configurations
  - Namespace definition
  - Network policies
  - Istio integration (PeerAuthentication, Sidecar, VirtualService)
  - Patches for resource customization
  - Additional systemd log collection configuration

### Key Configuration Patterns

1. **Log Collection Flow**:
   - DaemonSet mounts `/var/log` to collect container logs
   - Uses tail input plugin with Kubernetes metadata enrichment
   - Forwards compressed logs to aggregator on port 24224

2. **Log Aggregation**:
   - StatefulSet with persistent storage (1Gi)
   - Custom plugins for secret redaction, JSON parsing, and metadata enrichment
   - Outputs to Grafana Loki as primary storage
   - Prometheus metrics exposed on port 24231

3. **High Availability**:
   - HorizontalPodAutoscaler for aggregator (1-3 replicas)
   - PodDisruptionBudget ensures minimum availability
   - Nginx proxy provides load balancing across aggregator instances

4. **Security Features**:
   - Non-privileged containers with read-only root filesystem
   - Security contexts with proper user/group IDs
   - Automatic secret redaction for GitHub tokens and OpenAI API keys
   - Network policies in dev environment

### Important Configuration Files

- **fluentd-forward-fluent.conf** - Main configuration for log collectors
- **fluentd-aggregator-fluent.conf** - Main configuration for aggregators
- **kubernetes.conf** - Kubernetes-specific parsing rules
- **tail_container_parse.conf** - Container log parsing patterns
- **metrics.conf** - Prometheus metrics definitions and log-based monitoring
- **nginx.conf** - TCP proxy configuration for load balancing

### Development Tips

1. **Testing Configuration Changes**:
   - Use `kubectl kustomize` to preview changes before applying
   - ConfigMaps are immutable; changes require new generation
   - Check logs immediately after deployment for configuration errors

2. **Debugging Issues**:
   - Aggregator includes liveness probe that restarts every 19-24 hours
   - Check `/fluentd/buffer` in aggregator pods for buffered data
   - Prometheus metrics available at `http://fluentd-aggregator:24231/metrics`

3. **Custom Plugin Development**:
   - Custom plugins are in `/opt/hippocampus/cluster/applications/fluentd-aggregator/plugins/`
   - Rebuild aggregator image after plugin changes
   - Test plugins locally before deploying to cluster