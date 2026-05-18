#!/usr/bin/env -S bash -l

set -eo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

minikube ssh -- "sudo systemctl restart kubelet"

until minikube kubectl -- get --raw /readyz > /dev/null 2>&1; do
  sleep 5
done

minikube kubectl -- wait --for=condition=Ready node/minikube --timeout=300s

minikube kubectl -- delete validatingwebhookconfiguration -l release!=istio
minikube kubectl -- delete mutatingwebhookconfiguration -l release!=istio

if [ -n "$(minikube kubectl -- get pod -l app.kubernetes.io/name=cilium-agent -n kube-system -o name 2> /dev/null)" ]; then
  minikube kubectl -- delete pod -l app.kubernetes.io/name=cilium-agent -n kube-system
fi
if [ -n "$(minikube kubectl -- get pod -l k8s-app=istio-cni-node -n istio-system -o name 2> /dev/null)" ]; then
  minikube kubectl -- delete pod -l k8s-app=istio-cni-node -n istio-system
  sleep 3
  minikube kubectl -- wait --for=condition=Ready pod -l k8s-app=istio-cni-node -n istio-system --timeout=30m
fi
if [ -n "$(minikube kubectl -- get pod -l app=istiod -n istio-system -o name 2> /dev/null)" ]; then
  minikube kubectl -- delete pod -l app=istiod -n istio-system
  sleep 3
  minikube kubectl -- wait --for=condition=Ready pod -l app=istiod -n istio-system --timeout=30m
fi
if [ -n "$(minikube kubectl -- get pod -l app=ztunnel -n istio-system -o name 2> /dev/null)" ]; then
  minikube kubectl -- delete pod -l app=ztunnel -n istio-system
fi
if [ -n "$(minikube kubectl -- get pod -l operator.istio.io/component=IngressGateways -n istio-system -o name 2> /dev/null)" ]; then
  minikube kubectl -- delete pod -l operator.istio.io/component=IngressGateways -n istio-system
fi
if [ -n "$(minikube kubectl -- get pod -l operator.istio.io/component=EgressGateways -n istio-system -o name 2> /dev/null)" ]; then
  minikube kubectl -- delete pod -l operator.istio.io/component=EgressGateways -n istio-system
fi

minikube kubectl -- delete pod --field-selector=metadata.namespace!=istio-system --all-namespaces --force --grace-period=0
minikube ssh -- sudo cat /etc/kubernetes/addons/storage-provisioner.yaml | minikube kubectl -- apply -f -
