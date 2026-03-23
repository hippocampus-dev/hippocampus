# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Tempo is a distributed tracing backend by Grafana Labs deployed in this Hippocampus cluster. It collects traces via OpenTelemetry protocol, generates metrics from traces, and visualizes service dependencies.

## Common Development Commands

### Deployment Commands
- `kubectl apply -k overlays/dev/` - Deploy to development environment
- `kubectl delete -k overlays/dev/` - Remove from development environment
- `kubectl -n tempo rollout restart deployment -l app.kubernetes.io/name=tempo` - Restart all Tempo deployments
- `kubectl -n tempo rollout restart statefulset -l app.kubernetes.io/name=tempo` - Restart all Tempo statefulsets

### Debugging Commands
- `kubectl logs -n tempo -l app.kubernetes.io/name=tempo,app.kubernetes.io/component=distributor` - View distributor logs
- `kubectl logs -n tempo -l app.kubernetes.io/name=tempo,app.kubernetes.io/component=ingester` - View ingester logs
- `kubectl logs -n tempo -l app.kubernetes.io/name=tempo,app.kubernetes.io/component=query-frontend` - View query frontend logs
- `kubectl port-forward -n tempo svc/tempo-query-frontend 3100:3100` - Access Tempo query API locally
- `kubectl port-forward -n tempo svc/tempo-distributor 4317:4317` - Access OTLP receiver locally

### Trace Query Commands
- `curl http://localhost:3100/api/traces/{traceID}` - Query a specific trace (after port-forward)
- `curl http://localhost:3100/api/search -d '{"limit": 20}'` - Search recent traces

## High-Level Architecture

### Microservices Architecture
Tempo uses a scalable microservices architecture with 6 main components:

1. **Distributor** (Deployment) - Receives traces via OTLP on port 4317, performs rate limiting, and forwards to ingesters
2. **Ingester** (StatefulSet) - Temporarily stores traces in memory with WAL, builds blocks for object storage
3. **Querier** (Deployment) - Searches traces from both ingesters and object storage
4. **Query Frontend** (Deployment) - HTTP API endpoint on port 3100, optimizes and routes queries
5. **Compactor** (StatefulSet) - Compresses old trace data and maintains 3-hour retention
6. **Metrics Generator** (Deployment) - Generates service graphs and span metrics from traces, sends to Mimir

### Storage Architecture
- **Object Storage**: MinIO (S3-compatible) stores traces in Parquet v2 format
- **Caching**: Memcached via mcrouter caches trace blocks for up to 48 hours
- **Temporary Storage**: Memory-based emptyDir volumes for ingester WAL
- **Cluster Coordination**: Memberlist gossip protocol on port 7946

### Kustomize Structure
```
base/                         # Base manifests
├── deployment.yaml          # Distributor, Querier, Query Frontend, Metrics Generator
├── stateful_set.yaml        # Ingester, Compactor
├── service.yaml             # Services for each component
└── horizontal_pod_autoscaler.yaml

overlays/dev/                # Development overlay
├── files/tempo.yaml         # Main Tempo configuration
├── memcached/              # Memcached dependency
├── minio/                  # MinIO dependency
└── patches/                # Resource patches
```

### Security Patterns
- Non-root execution (UID: 65532)
- Read-only root filesystem
- All capabilities dropped
- Istio sidecar injection for mTLS
- Service mesh policies (PeerAuthentication, Sidecar, Telemetry)

### Performance Configuration
- GOMAXPROCS/GOMEMLIMIT auto-configuration
- Max 400 concurrent workers
- Queue depth of 20,000
- Rate limiting: 15MB/s with 20MB burst
- Block polling disabled for resource efficiency

### Integration Points
- **Trace Input**: OpenTelemetry Agent at `otel-agent.otel.svc.cluster.local:4317`
- **Metrics Output**: Mimir at `mimir-distributor.mimir.svc.cluster.local:8080`
- **Visualization**: Grafana for trace exploration
- **Internal Tracing**: Jaeger at `jaeger-collector.jaeger.svc.cluster.local:14268`

### ArgoCD Deployment Notes
- MinIO deploys first with sync-wave `-2`
- All ConfigMaps are immutable
- Images pulled from GitHub Container Registry mirror
- Namespace `tempo` must exist before deployment