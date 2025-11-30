# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Common Development Commands

### Building and Deploying
```bash
# Build kustomization for dev environment
kubectl kustomize overlays/dev

# Apply to cluster
kubectl apply -k overlays/dev

# Delete deployment
kubectl delete -k overlays/dev

# Watch deployment status
kubectl -n adhoc get pods -l app.kubernetes.io/name=adhoc -w

# Check Elasticsearch cluster health
kubectl -n adhoc exec -it adhoc-elasticsearch-master-0 -- curl -s localhost:9200/_cluster/health?pretty
```

### Verifying Configuration
```bash
# Check generated manifests without applying
kubectl kustomize . | less

# Validate YAML syntax
kubectl kustomize . | kubectl apply --dry-run=client -f -

# Build from a specific overlay
kubectl kustomize overlays/dev | less
```

### Debugging and Monitoring
```bash
# View logs for specific node type
kubectl -n adhoc logs -l app.kubernetes.io/component=elasticsearch-master -f
kubectl -n adhoc logs -l app.kubernetes.io/component=elasticsearch-data -f
kubectl -n adhoc logs -l app.kubernetes.io/component=elasticsearch-ingest -f

# Check initialization job status
kubectl -n adhoc get job adhoc-curl -o yaml

# Access Elasticsearch API
kubectl -n adhoc port-forward svc/adhoc-elasticsearch 9200:9200
```

## High-Level Architecture

### Elasticsearch Cluster Structure
This deployment creates a multi-node Elasticsearch cluster with specialized node roles:
- **Master nodes** (3x): Cluster state management, fixed count for quorum
- **Data nodes** (1x): Data storage, uses persistent volumes (10Gi)
- **Ingest nodes** (1-5x): Data preprocessing, autoscaling enabled
- **Coordinating nodes** (1-5x): Request routing, autoscaling enabled

Each node type has specific resource allocations and Java heap settings configured via environment variables.

### Kustomization Architecture
The deployment uses a layered Kustomize approach:
1. **Base resources**: References `../../utilities/elasticsearch` for shared Elasticsearch configuration
2. **Patches**: Modifies base resources for adhoc-specific requirements
3. **Overlays**: Environment-specific configurations (currently only `dev`)

Key customizations:
- Istio service mesh integration with mTLS (STRICT mode, except port 9300 for discovery)
- Horizontal pod autoscaling for stateless nodes (1-5 replicas based on CPU/memory)
- Pod disruption budgets for high availability
- Network policies for security isolation (default deny with specific allow rules)
- Prometheus metrics integration via elasticsearch-exporter sidecars

### Configuration Management
- **elasticsearch.yml**: Core Elasticsearch settings (security disabled for dev)
- **pipeline.yaml**: Ingest pipeline for JSON field processing
- **template.json**: Index template for `events-logger-*` indices
- **kustomizeconfig.yaml**: Defines replacement targets for node counts

The deployment dynamically propagates master/data node counts throughout all resources using Kustomize replacements, ensuring consistency across StatefulSets, Services, and configuration.

### Network and Security
- **Network Policy**: Default deny with specific ingress rules for:
  - Elasticsearch inter-node communication (port 9300)
  - HTTP API access from Grafana (port 9200)
  - Log ingestion from Fluentd to ingest nodes
  - Prometheus metrics scraping (port 15020)
- **Istio Integration**: 
  - Sidecar injection enabled with resource limits
  - PeerAuthentication for mTLS enforcement
  - Telemetry configuration for metrics collection

### Initialization
The `adhoc-curl` Job (in overlays/dev) runs after deployment to:
- Wait for Elasticsearch cluster to be ready
- Create index templates from `template.json`
- Set up ingest pipelines from `pipeline.yaml`