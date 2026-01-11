#!/usr/bin/env -S bash -l

set -e

if kubectl config get-contexts hippocampus; then
  kubectl config delete-context hippocampus
fi

# Disable minikube detection of skaffold
kubectl config rename-context minikube hippocampus
