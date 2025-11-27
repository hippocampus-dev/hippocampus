# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Kustomize-based Kubernetes manifest repository for deploying Knative Eventing with Kafka integration. It provides event-driven architecture support using Knative's eventing system backed by Apache Kafka (deployed via Strimzi operator).

## Common Development Commands

### Working with Kustomize
- `kubectl kustomize overlays/dev` - Build and preview the development overlay
- `kubectl apply -k overlays/dev` - Apply the development overlay to your cluster
- `kubectl diff -k overlays/dev` - Check what changes would be applied

### Deployment Commands
- Deploy to dev environment: `kubectl apply -k overlays/dev -n knative-eventing`
- Check deployment status: `kubectl get pods -n knative-eventing`
- View Kafka cluster status: `kubectl get kafka -n knative-eventing`

## High-Level Architecture

### Manifest Structure
This repository uses Kustomize for managing Kubernetes manifests with the following structure:

1. **Base Layer** (`base/`):
   - References upstream Knative Eventing Kafka resources (v1.14.7)
   - Includes controller, broker, channel, sink, and source components

2. **Development Overlay** (`overlays/dev/`):
   - Deploys a Strimzi Kafka cluster with KRaft mode enabled
   - Configures Knative components with Istio sidecar injection
   - Sets up dead-letter queue pattern with KafkaSource and KafkaSink
   - Includes CloudEvents logger for debugging

### Key Components

1. **Kafka Infrastructure**:
   - Strimzi-managed Kafka cluster (3 replicas, version 3.9.0)
   - KRaft mode enabled (no ZooKeeper)
   - Entity operators for topic and user management
   - Kafka Exporter for Prometheus metrics

2. **Knative Eventing Components**:
   - Kafka Controller - manages Kafka-based eventing resources
   - Kafka Broker Receiver/Dispatcher - handles broker pattern
   - Kafka Channel Receiver/Dispatcher - handles channel pattern
   - Kafka Sink Receiver - handles sink pattern
   - Kafka Webhook - validates and mutates Kafka resources

3. **Event Flow Pattern**:
   - Dead-letter topic configured for failed events
   - KafkaSource reads from dead-letter topic
   - Events routed through channels and subscriptions
   - CloudEvents logger broker for debugging

### Deployment Patterns

1. **Resource Ordering**: Uses ArgoCD sync-wave annotations for proper resource creation order
2. **High Availability**: Topology spread constraints for zone and node distribution
3. **Observability**: Prometheus annotations on all components for metrics collection
4. **Service Mesh**: Istio sidecar injection with resource limits configured

### Integration Points

- Integrates with `cloudevents-logger` application (from `../../../../applications/cloudevents-logger/manifests`)
- Expects Knative Eventing core components to be installed
- Requires Strimzi operator for Kafka management
- Works with Istio service mesh for traffic management