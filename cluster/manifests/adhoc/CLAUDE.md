# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Common Development Commands

### Kustomize Build and Deploy
```bash
# View generated manifests without applying
kubectl kustomize overlays/dev

# Deploy to development environment
kubectl apply -k overlays/dev

# Delete deployment
kubectl delete -k overlays/dev

# Validate YAML syntax before applying
kubectl kustomize overlays/dev | kubectl apply --dry-run=client -f -
```

### Monitoring and Debugging
```bash
# Watch pod status
kubectl -n adhoc get pods -l app.kubernetes.io/name=adhoc -w

# View logs for specific Elasticsearch node types
kubectl -n adhoc logs -l app.kubernetes.io/component=elasticsearch-master -f
kubectl -n adhoc logs -l app.kubernetes.io/component=elasticsearch-data -f
kubectl -n adhoc logs -l app.kubernetes.io/component=elasticsearch-ingest -f
kubectl -n adhoc logs -l app.kubernetes.io/component=elasticsearch-coordinating -f

# Check Elasticsearch cluster health
kubectl -n adhoc exec -it adhoc-elasticsearch-master-0 -- curl -s localhost:9200/_cluster/health?pretty

# Check initialization job status
kubectl -n adhoc get job adhoc-curl -o yaml
kubectl -n adhoc logs job/adhoc-curl

# Port forward for local access
kubectl -n adhoc port-forward svc/adhoc-elasticsearch 9200:9200
```

## High-Level Architecture

### Directory Structure
The adhoc directory contains experimental or temporary Kubernetes deployments, currently hosting an Elasticsearch cluster configuration:

```
adhoc/
├── elasticsearch/       # Elasticsearch base configuration
│   ├── files/          # Configuration files (elasticsearch.yml, pipeline.yaml, template.json)
│   ├── patches/        # Kustomize patches for customization
│   └── *.yaml          # Istio and Kubernetes resource definitions
└── overlays/
    └── dev/            # Development environment overlay
```

### Elasticsearch Cluster Architecture
This deployment creates a multi-node Elasticsearch cluster with specialized roles:
- **Master nodes (3x)**: Fixed count for quorum, manages cluster state
- **Data nodes (1x)**: Stores data with 10Gi persistent volumes
- **Ingest nodes (1-5x)**: Preprocesses documents, autoscaling enabled
- **Coordinating nodes (1-5x)**: Routes requests, autoscaling enabled

### Kustomization Strategy
1. **Base resources**: References `../../utilities/elasticsearch` for shared configurations
2. **Patches**: Applies adhoc-specific modifications
3. **Overlays**: Environment-specific settings (development only)

Key features:
- Dynamic node count propagation via Kustomize replacements
- Istio service mesh integration with mTLS (STRICT mode)
- Horizontal pod autoscaling based on CPU/memory metrics
- Pod disruption budgets for high availability
- Network policies with default-deny and specific allow rules

### Security and Networking
- **Network Policy**: 
  - Default deny all traffic
  - Allows: Elasticsearch inter-node (9300), HTTP API from Grafana (9200), Fluentd logs ingestion, Prometheus metrics (15020)
- **Istio Integration**:
  - PeerAuthentication for mTLS enforcement (except port 9300 for node discovery)
  - Sidecar resource limits configuration
  - Telemetry for metrics collection

### Initialization Process
The `adhoc-curl` Job automatically:
1. Waits for Elasticsearch readiness
2. Creates index templates from `files/template.json`
3. Sets up ingest pipelines from `files/pipeline.yaml`