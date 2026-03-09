#!/usr/bin/env -S bash -l

set -e
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

if kubectl config get-contexts hippocampus; then
  kubectl config delete-context hippocampus
fi

# Disable minikube detection of skaffold
kubectl config rename-context minikube hippocampus
