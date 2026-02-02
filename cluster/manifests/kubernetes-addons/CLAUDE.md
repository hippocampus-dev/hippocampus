# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

This directory contains Kubernetes service discovery endpoints for core Kubernetes components' metrics collection. These services enable Prometheus and other monitoring tools to scrape metrics from Kubernetes control plane components.

## Services Provided

- **kube-proxy-discovery** - Exposes kube-proxy metrics on port 10249
- **kube-controller-manager-discovery** - Exposes controller-manager metrics on port 10257
- **kube-scheduler-discovery** - Exposes scheduler metrics on port 10259
- **kube-etcd-discovery** - Exposes etcd metrics on port 2381

## Common Development Commands

### Deploy to Development Environment
```bash
# Apply the manifests
kubectl apply -k overlays/dev/

# Preview the generated manifests
kubectl kustomize overlays/dev/

# Check if services are created
kubectl get services -n kube-system | grep discovery
```

### Verify Service Endpoints
```bash
# Check if the services have endpoints
kubectl get endpoints -n kube-system kube-proxy-discovery
kubectl get endpoints -n kube-system kube-controller-manager-discovery
kubectl get endpoints -n kube-system kube-scheduler-discovery
kubectl get endpoints -n kube-system kube-etcd-discovery
```

## Architecture

The directory follows the standard Kustomize base/overlays pattern:

- `base/` - Contains the core service definitions
  - `service.yaml` - Defines all four discovery services
  - `kustomization.yaml` - Base Kustomize configuration
- `overlays/dev/` - Development environment overlay
  - Applies resources to the `kube-system` namespace

These services use label selectors to find their target pods:
- kube-proxy: `k8s-app: kube-proxy`
- kube-controller-manager: `component: kube-controller-manager`
- kube-scheduler: `component: kube-scheduler`
- etcd: `component: etcd`

## Integration with Monitoring Stack

These discovery services are typically used by Prometheus ServiceMonitor resources to enable metrics collection from Kubernetes core components. The monitoring stack (Prometheus, Grafana) in the parent manifests directory relies on these endpoints for cluster health monitoring and alerting.