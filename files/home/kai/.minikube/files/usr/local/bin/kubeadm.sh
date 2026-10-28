#!/usr/bin/env bash

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

# https://github.com/kubernetes/minikube/blob/v1.38.1/pkg/minikube/bootstrapper/kubeadm/kubeadm.go#L419-L426
if [ -d /var/lib/minikube/etcd/member ] && { [ "$1" = "reset" ] || { [ "$1" = "init" ] && [ "$2" != "phase" ]; }; }; then
  echo "refusing kubeadm $1: /var/lib/minikube/etcd holds the cluster's data" >&2
  exit 1
fi

exec "${0}.bak" "$@"
