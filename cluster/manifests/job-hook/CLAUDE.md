# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This directory contains Kubernetes manifests for the job-hook service, a mutating admission webhook that automatically injects creation timestamps into Job resources. The manifests follow Kustomize patterns with base resources and environment-specific overlays.

## Common Development Commands

### Deployment and Testing
- `kubectl apply -k overlays/dev/` - Deploy to dev environment using Kustomize
- `kubectl delete -k overlays/dev/` - Remove deployment from dev environment
- `kubectl get all -n job-hook` - View all resources in job-hook namespace
- `kubectl logs -n job-hook deployment/job-hook` - Check webhook logs
- `kubectl describe mutatingwebhookconfigurations job-hook` - Verify webhook configuration

### Kustomize Operations
- `kubectl kustomize overlays/dev/` - Preview generated manifests without applying
- `kustomize build overlays/dev/` - Build and output final manifests

### Verification
- Create a test Job and verify annotation: `kubectl get job <job-name> -o jsonpath='{.spec.template.metadata.annotations.job-hook\.kaidotio\.github\.io/job-creation-timestamp}'`

## High-Level Architecture

### Manifest Structure
1. **Base Layer** (`base/`):
   - References the application manifests from `/cluster/applications/job-hook/manifests`
   - Provides foundation for all environments

2. **Dev Overlay** (`overlays/dev/`):
   - Namespace configuration
   - Network policies for pod-to-pod communication
   - Istio integration (PeerAuthentication, Sidecar, Telemetry)
   - Patches for environment-specific configurations

3. **Key Resources**:
   - **Certificate**: TLS certificate managed by cert-manager for webhook HTTPS
   - **MutatingWebhookConfiguration**: Configures Kubernetes to call the webhook for Job creation
   - **Deployment**: 2-replica deployment with security hardening
   - **Service**: ClusterIP service exposing webhook endpoint
   - **PodDisruptionBudget**: Ensures availability during cluster operations

### Integration Points
- **cert-manager**: Provides TLS certificates for webhook HTTPS endpoint
- **Istio**: Service mesh integration for mTLS and observability
- **Kubernetes API**: Receives admission requests for batch/v1 Job resources

### Security Configuration
- Runs with strict security context (non-root, read-only filesystem)
- Network policies restrict traffic to required connections only
- Istio PeerAuthentication enforces mTLS for pod-to-pod communication