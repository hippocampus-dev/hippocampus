# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This directory contains Kubernetes manifests for deploying http-kvs, a simple HTTP-based key-value store that uses S3 (or S3-compatible storage) as its backend. The manifests follow the Kustomize pattern with base configurations and environment-specific overlays.

## Directory Structure

```
http-kvs/
├── base/                    # Base Kubernetes resources
│   ├── deployment.yaml      # Main http-kvs deployment
│   ├── horizontal_pod_autoscaler.yaml  # HPA configuration
│   ├── kustomization.yaml   # Base kustomization
│   ├── pod_disruption_budget.yaml      # PDB for high availability
│   └── service.yaml         # Service definition
└── overlays/
    └── dev/                 # Development environment overlay
        ├── authorization_policy.yaml    # Istio authorization rules
        ├── gateway.yaml     # Istio gateway configuration
        ├── job.yaml         # MinIO bucket creation job
        ├── kustomization.yaml           # Dev overlay kustomization
        ├── minio/           # MinIO storage configuration
        │   ├── kustomization.yaml
        │   └── patches/     # MinIO-specific patches
        ├── namespace.yaml   # Namespace definition
        ├── network_policy.yaml          # Network segmentation rules
        ├── patches/         # Environment-specific patches
        ├── peer_authentication.yaml     # mTLS configuration
        ├── sidecar.yaml     # Istio sidecar configuration
        ├── telemetry.yaml   # Telemetry/metrics configuration
        └── virtual_service.yaml         # Istio routing rules
```

## Key Components

### Base Configuration
- **Deployment**: Runs http-kvs with security context, resource management, and health checks
- **Service**: Exposes http-kvs on port 8080
- **HPA**: Provides automatic scaling based on CPU/memory metrics
- **PDB**: Ensures minimum availability during updates/disruptions

### Development Overlay
- **MinIO Integration**: Uses MinIO as S3-compatible storage backend
- **Istio Service Mesh**: Full integration with Istio for mTLS, observability, and traffic management
- **Network Policies**: Enforces network segmentation and security
- **Topology Spread**: Ensures pods are distributed across nodes and zones

## Common Development Commands

### Viewing Rendered Manifests
```bash
# View base manifests
kubectl kustomize base/

# View dev environment manifests
kubectl kustomize overlays/dev/

# View specific resource types
kubectl kustomize overlays/dev/ | grep -A20 "kind: Deployment"
```

### Deployment Commands
```bash
# Deploy to dev environment
kubectl apply -k overlays/dev/

# Check deployment status
kubectl -n http-kvs get all

# View logs
kubectl -n http-kvs logs -l app.kubernetes.io/name=http-kvs

# Check MinIO status
kubectl -n http-kvs get statefulset http-kvs-minio
```

### Testing the Service
```bash
# Port-forward for local testing
kubectl -n http-kvs port-forward service/http-kvs 8080:8080

# Test endpoints
curl -X POST http://localhost:8080/mykey -d "myvalue"
curl http://localhost:8080/mykey
curl -X DELETE http://localhost:8080/mykey
curl http://localhost:8080/healthz
```

## Architecture Details

### Security Features
1. **Pod Security**:
   - Non-root user (65532)
   - Read-only root filesystem
   - No privilege escalation
   - All capabilities dropped
   - Seccomp profile enabled

2. **Network Security**:
   - Network policies restrict ingress/egress
   - Istio mTLS for service-to-service communication
   - Authorization policies for access control

3. **Service Mesh Integration**:
   - Istio sidecar injection enabled
   - Telemetry collection configured
   - Virtual service for traffic routing
   - Gateway for external access

### High Availability
- **PodDisruptionBudget**: Ensures minimum availability
- **Topology Spread Constraints**: Distributes pods across failure domains
- **Horizontal Pod Autoscaler**: Scales based on load
- **Rolling Update Strategy**: Zero-downtime deployments

### Storage Backend
- **MinIO**: S3-compatible object storage for development
- **Bucket Creation**: Automated via Kubernetes Job
- **Credentials**: Configured via environment variables

## Environment Variables

The deployment patches configure these environment variables:
- `S3_BUCKET`: http-kvs
- `S3_ENDPOINT_URL`: http://http-kvs-minio.http-kvs.svc.cluster.local:9000
- `AWS_ACCESS_KEY_ID`: minio
- `AWS_SECRET_ACCESS_KEY`: miniominio

## Monitoring and Observability

1. **Health Checks**:
   - Readiness probe on `/healthz`
   - Liveness probe configuration

2. **Metrics**:
   - Prometheus scraping enabled via Istio
   - Telemetry configuration for metrics collection

3. **Resource Management**:
   - GOMAXPROCS set based on CPU limits
   - GOMEMLIMIT set based on memory limits

## Best Practices When Modifying

1. **Always use Kustomize**: Never modify base resources directly for environment-specific changes
2. **Security First**: Maintain security contexts and network policies
3. **Resource Limits**: Ensure proper resource requests/limits for HPA
4. **Health Checks**: Keep readiness/liveness probes configured
5. **Istio Integration**: Maintain service mesh configuration for observability

## Troubleshooting

### Common Issues

1. **MinIO Connection Failed**:
   ```bash
   # Check MinIO pod status
   kubectl -n http-kvs get pod -l app.kubernetes.io/name=http-kvs-minio
   
   # Check MinIO logs
   kubectl -n http-kvs logs -l app.kubernetes.io/name=http-kvs-minio
   ```

2. **Pod Not Starting**:
   ```bash
   # Check events
   kubectl -n http-kvs describe pod -l app.kubernetes.io/name=http-kvs
   
   # Check resource availability
   kubectl -n http-kvs describe hpa http-kvs
   ```

3. **Network Issues**:
   ```bash
   # Check network policies
   kubectl -n http-kvs get networkpolicy
   
   # Check Istio configuration
   kubectl -n http-kvs get virtualservice,destinationrule,gateway
   ```

## Related Documentation
- Application source code: `/opt/hippocampus/cluster/applications/http-kvs/`
- MinIO utilities: `/opt/hippocampus/cluster/manifests/utilities/minio/`
- Parent manifest guidelines: `/opt/hippocampus/cluster/manifests/README.md`