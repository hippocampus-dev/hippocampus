# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Component Overview

cilium-etcd is a dedicated etcd instance for the Cilium CNI (Container Network Interface) plugin. It runs as a StatefulSet in the `kube-system` namespace and stores Cilium's network policies, endpoint information, and cluster-wide network state separately from the main Kubernetes etcd cluster.

## Common Development Commands

### Kubernetes Operations
- `kubectl apply -k overlays/dev/` - Apply the development overlay configuration
- `kubectl -n kube-system get statefulset cilium-etcd -o yaml` - View the current StatefulSet configuration
- `kubectl -n kube-system logs cilium-etcd-0` - View etcd logs
- `kubectl -n kube-system exec -it cilium-etcd-0 -- etcdctl member list` - Check etcd cluster membership
- `kubectl -n kube-system port-forward cilium-etcd-0 12379:12379` - Forward etcd client port for local access

### Testing etcd Health
- `kubectl -n kube-system exec cilium-etcd-0 -- etcdctl --endpoints=http://127.0.0.1:12379 endpoint health` - Check endpoint health
- `kubectl -n kube-system exec cilium-etcd-0 -- etcdctl --endpoints=http://127.0.0.1:12379 endpoint status --write-out=table` - View detailed status

### Monitoring
- Access metrics at `http://<pod-ip>:12381/metrics` after port-forwarding or through Prometheus

## Architecture and Configuration Patterns

### Kustomize Structure
- **base/**: Core StatefulSet, Service, and PodDisruptionBudget definitions
- **overlays/dev/**: Development-specific patches (e.g., Prometheus annotations, topology constraints)

### Key Configuration Details
- Uses `hostNetwork: true` since it's a CNI component that needs direct network access
- Data persisted to host path `/var/lib/cilium/etcd-0`
- Ports: 12379 (client), 12380 (peer), 12381 (metrics)
- Security hardening: read-only root filesystem, dropped capabilities, non-privileged container

### Image Management
- Base image: `quay.io/coreos/etcd`
- Uses GitHub Container Registry mirror: `ghcr.io/hippocampus-dev/hippocampus/mirror/quay.io/coreos/etcd`
- Version pinned by SHA256 digest for reproducibility

### GitOps Integration
- Deployed via ArgoCD from `/cluster/manifests/argocd-applications/base/cilium-etcd.yaml`
- Auto-sync enabled with self-healing
- Slack notifications configured for sync failures

## Development Notes

### When Making Changes
1. Modify base configuration for changes that apply to all environments
2. Use overlays for environment-specific adjustments
3. Ensure image digests are updated when upgrading etcd versions
4. Test StatefulSet rolling updates carefully as etcd is stateful

### Common Scenarios
- **Scaling**: Currently single-instance; scaling requires updating initial cluster configuration
- **Backup**: Consider implementing etcd snapshots for production use
- **TLS**: Currently uses plaintext; production deployments should enable TLS

### Integration Points
- Cilium agent pods connect to this etcd instance for storing network state
- Prometheus scrapes metrics from port 12381
- Must be running before Cilium agents can start successfully