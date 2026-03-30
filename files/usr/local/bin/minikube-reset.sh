#!/usr/bin/env -S bash -l

set -eo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

declare -A components=(
  [kube-apiserver]=8443
  [kube-controller]=10257
  [kube-scheduler]=10259
  [etcd]=2379
)

for name in "${!components[@]}"; do
  port=${components[${name}]}

  pid=$(minikube ssh -- "sudo ss -tlnp 'sport = :${port}'" | tr -d '\r' | awk -F'pid=|,' "/${name}/{print \$3}")
  if [ -z "$pid" ] || ! [[ "$pid" =~ ^[0-9]+$ ]]; then
    continue
  fi

  container_state=$(minikube ssh -- "sudo crictl ps --name $name -o json" | jq -r '.containers[0].state // empty')
  if [ "$container_state" = "CONTAINER_RUNNING" ]; then
    continue
  fi

  minikube ssh -- "sudo kill -9 $pid"
done

minikube ssh -- "sudo systemctl restart kubelet"

until minikube kubectl -- get --raw /readyz > /dev/null 2>&1; do
  sleep 5
done

minikube kubectl -- wait --for=condition=Ready node/minikube --timeout=300s

minikube kubectl -- delete validatingwebhookconfiguration -l release!=istio
minikube kubectl -- delete mutatingwebhookconfiguration -l release!=istio

minikube kubectl -- delete pod -l app.kubernetes.io/name=cilium-agent -n kube-system
minikube kubectl -- delete pod -l k8s-app=istio-cni-node -n istio-system
sleep 3
minikube kubectl -- wait --for=condition=Ready pod -l k8s-app=istio-cni-node -n istio-system --timeout=30m
minikube kubectl -- delete pod -l app=istiod -n istio-system
sleep 3
minikube kubectl -- wait --for=condition=Ready pod -l app=istiod -n istio-system --timeout=30m
minikube kubectl -- delete pod -l app=ztunnel -n istio-system
minikube kubectl -- delete pod -l operator.istio.io/component=IngressGateways -n istio-system
minikube kubectl -- delete pod -l operator.istio.io/component=EgressGateways -n istio-system

minikube kubectl -- delete pod --field-selector=metadata.namespace!=istio-system --all-namespaces --force --grace-period=0
minikube ssh -- sudo cat /etc/kubernetes/addons/storage-provisioner.yaml | minikube kubectl -- apply -f -
