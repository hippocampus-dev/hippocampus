# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

This directory contains Kubernetes manifests for deploying Knative Eventing components in the Hippocampus platform. The manifests use Kustomize for environment-specific configurations and integrate with Istio service mesh for traffic management and observability.

## Common Development Commands

### Deploying the manifests
```bash
# Deploy to development environment
kubectl apply -k overlays/dev

# View the generated manifests without applying
kubectl kustomize overlays/dev

# Delete the resources
kubectl delete -k overlays/dev
```

### Working with Kustomize patches
```bash
# Check if patches are valid
kubectl kustomize overlays/dev --enable-alpha-plugins

# Create a new patch
kubectl create deployment example --dry-run=client -o yaml > overlays/dev/patches/new-patch.yaml
```

## Architecture and Structure

### Directory Layout
- **base/** - Contains the base Kustomization that pulls the upstream Knative Eventing release
- **overlays/dev/** - Development environment customizations including:
  - Network policies for pod-to-pod communication
  - Istio sidecar configurations for service mesh integration
  - Deployment patches for resource limits, topology spread, and monitoring
  - PeerAuthentication for mTLS enforcement
  - Telemetry configuration for metrics collection

### Key Components Configured

1. **Core Eventing Components**:
   - eventing-controller - Main controller for managing eventing resources
   - eventing-webhook - Admission webhook for validation

2. **Kafka Integration Components**:
   - kafka-broker-receiver/dispatcher - Handles Kafka broker events
   - kafka-channel-receiver/dispatcher - Manages Kafka channel events
   - kafka-sink-receiver - Processes Kafka sink events
   - kafka-source-dispatcher - Dispatches events from Kafka sources
   - kafka-controller - Controls Kafka-related resources
   - kafka-eventing-webhook - Validates Kafka eventing resources

3. **Additional Components**:
   - pingsource-mt-adapter - Multi-tenant adapter for ping sources
   - cloudevents-logger - Logging service for CloudEvents

### Integration Patterns

1. **Istio Service Mesh**: All components have Sidecar configurations controlling egress traffic:
   - Controllers use REGISTRY_ONLY mode for security
   - Dispatchers use ALLOW_ANY for external event delivery
   - All components can reach Istio control plane and telemetry services

2. **Network Policies**: Zero-trust networking with:
   - Default deny-all policy
   - Explicit ingress rules for receivers (port 8080)
   - Prometheus scraping allowed from the prometheus namespace
   - Knative Services (cloudevents-logger) allow ingress from kafka-broker-dispatcher for direct mesh traffic when Pod is running

3. **Observability**:
   - Prometheus annotations for metrics scraping on port 9090
   - Istio telemetry configuration for distributed tracing
   - Resource limits and requests for predictable performance

4. **High Availability**:
   - Topology spread constraints for zone and node distribution
   - Rolling update strategies with controlled surge/unavailability
   - Horizontal pod autoscaling patches available

### Environment-Specific Considerations

The dev overlay adds:
- Istio sidecar injection with specific resource limits
- Development-friendly replica counts
- Monitoring and debugging annotations
- Network policies suitable for development cluster

When creating new overlays for other environments (prod, staging), consider:
- Adjusting replica counts and resource limits
- Modifying topology spread constraints for your cluster topology
- Updating network policies for your security requirements
- Configuring appropriate autoscaling parameters
