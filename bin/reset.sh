#!/usr/bin/env bash

set -eo pipefail

kubectl delete pod -l k8s-app=istio-cni-node -n istio-system
kubectl wait --for=condition=Ready pod -l k8s-app=istio-cni-node -n istio-system --timeout=30m
kubectl delete pod -l app=istiod -n istio-system
kubectl wait --for=condition=Ready pod -l app=istiod -n istio-system --timeout=30m
kubectl delete pod -l app=ztunnel -n istio-system
kubectl delete pod -l operator.istio.io/component=IngressGateways -n istio-system
kubectl delete pod -l operator.istio.io/component=EgressGateways -n istio-system

kubectl delete pod --field-selector=metadata.namespace!=istio-system --all-namespaces --force --grace-period=0
minikube ssh -- sudo cat /etc/kubernetes/addons/storage-provisioner.yaml | kubectl apply -f -
