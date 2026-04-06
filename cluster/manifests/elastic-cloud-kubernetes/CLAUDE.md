# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with the Elastic Cloud on Kubernetes (ECK) manifests in this directory.

## Overview

This directory contains Kustomize manifests for deploying the Elastic Cloud on Kubernetes (ECK) operator. ECK extends Kubernetes with custom resources to deploy and manage Elasticsearch, Kibana, APM Server, Enterprise Search, Beats, Elastic Agent, Elastic Maps Server, and Logstash.

## Directory Structure

- **`base/`** - Base configuration that references upstream ECK resources
  - Downloads ECK CRDs and operator manifests from Elastic's official releases
  - Currently using ECK version 2.15.0

- **`overlays/dev/`** - Development environment customizations
  - Istio service mesh integration (sidecar injection, telemetry)
  - Network policies for security
  - Resource patches for the StatefulSet
  - Namespace configuration

## Key Components

### Base Layer
- References external ECK resources directly from Elastic's download site
- Includes both CRDs and operator deployment

### Dev Overlay Patches
- **`patches/stateful_set.yaml`** - Customizes the elastic-operator StatefulSet with:
  - Istio sidecar injection
  - Security context enhancements
  - Topology spread constraints for high availability
  - Resource limits for the sidecar proxy

- **`patches/namespace.yaml`** - Ensures elastic-system namespace exists

### Istio Integration
- **`sidecar.yaml`** - Configures Istio sidecar behavior
- **`peer_authentication.yaml`** - Sets up mTLS for pod-to-pod communication
- **`telemetry.yaml`** - Configures metrics and tracing

### Security
- **`network_policy.yaml`** - Implements default-deny with specific allow rules
  - Allows Prometheus scraping of Envoy metrics on port 15020

## Common Operations

### Deploy to Development
```bash
kustomize build overlays/dev | kubectl apply -f -
```

### Update ECK Version
1. Edit `base/kustomization.yaml`
2. Update the version numbers in the resource URLs
3. Test the deployment in a non-production environment first

### Customize for New Environment
1. Create a new overlay directory (e.g., `overlays/prod`)
2. Copy and modify the kustomization.yaml from dev
3. Adjust patches and resources as needed

## Important Considerations

1. **Version Compatibility**: Ensure ECK version is compatible with your Kubernetes version
2. **CRD Updates**: Be careful when updating CRDs as they affect all ECK resources in the cluster
3. **Istio Integration**: The dev overlay assumes Istio is installed and configured
4. **Network Policies**: The default-deny policy requires explicit allow rules for all communication

## Integration with Hippocampus

This ECK deployment integrates with the broader Hippocampus platform:
- Provides Elasticsearch backend for search and analytics
- Monitored via Prometheus through Istio metrics
- Follows the same Istio service mesh patterns as other services
- Uses consistent security and networking policies

## Troubleshooting

1. **Operator Not Starting**: Check if Istio sidecar is preventing startup
2. **Network Issues**: Verify network policies allow required communication
3. **Resource Constraints**: Ensure nodes have sufficient resources for ECK components
4. **CRD Conflicts**: Check for existing ECK installations before applying