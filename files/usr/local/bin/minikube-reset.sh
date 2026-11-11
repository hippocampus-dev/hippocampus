#!/usr/bin/env -S bash -l

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

minikube ssh -- "sudo systemctl restart kubelet"

until minikube kubectl -- get --raw /readyz > /dev/null 2>&1; do
  sleep 5
done

minikube kubectl -- wait --for=condition=Ready node/minikube --timeout=300s

minikube kubectl -- delete validatingwebhookconfiguration -l release!=istio
minikube kubectl -- get mutatingwebhookconfiguration -l release!=istio -o name \
  | awk '!/\/pods\.defaulting\.webhook\.kafka\.eventing\.knative\.dev$/ && !/\/relocatable-volume-controller$/' \
  | xargs -r minikube kubectl -- delete --ignore-not-found

# kubectl delete and wait block on a pod of a node without a kubelet until the node returns
ready_node_selector=$(minikube kubectl -- get nodes -o json | jq -r '[.items[] | select(.metadata.name != "minikube" and (any(.status.conditions[]; .type == "Ready" and .status == "True") | not)) | "spec.nodeName!=" + .metadata.name] | join(",")')

if [ -n "$(minikube kubectl -- get pod -l app.kubernetes.io/name=cilium-agent -n kube-system --field-selector="$ready_node_selector" -o name 2> /dev/null)" ]; then
  minikube kubectl -- delete pod -l app.kubernetes.io/name=cilium-agent -n kube-system --field-selector="$ready_node_selector"
fi
if [ -n "$(minikube kubectl -- get pod -l k8s-app=istio-cni-node -n istio-system --field-selector="$ready_node_selector" -o name 2> /dev/null)" ]; then
  minikube kubectl -- delete pod -l k8s-app=istio-cni-node -n istio-system --field-selector="$ready_node_selector"
  until minikube kubectl -- wait --for=condition=Ready pod -l k8s-app=istio-cni-node -n istio-system --field-selector="$ready_node_selector" --timeout=10m; do sleep 1; done
fi
if [ -n "$(minikube kubectl -- get pod -l app=istiod -n istio-system --field-selector="$ready_node_selector" -o name 2> /dev/null)" ]; then
  minikube kubectl -- delete pod -l app=istiod -n istio-system --field-selector="$ready_node_selector"
  until minikube kubectl -- wait --for=condition=Ready pod -l app=istiod -n istio-system --field-selector="$ready_node_selector" --timeout=10m; do sleep 1; done
fi
if [ -n "$(minikube kubectl -- get pod -l app=ztunnel -n istio-system --field-selector="$ready_node_selector" -o name 2> /dev/null)" ]; then
  minikube kubectl -- delete pod -l app=ztunnel -n istio-system --field-selector="$ready_node_selector"
fi
if [ -n "$(minikube kubectl -- get pod -l operator.istio.io/component=IngressGateways -n istio-system --field-selector="$ready_node_selector" -o name 2> /dev/null)" ]; then
  minikube kubectl -- delete pod -l operator.istio.io/component=IngressGateways -n istio-system --field-selector="$ready_node_selector"
fi
if [ -n "$(minikube kubectl -- get pod -l operator.istio.io/component=EgressGateways -n istio-system --field-selector="$ready_node_selector" -o name 2> /dev/null)" ]; then
  minikube kubectl -- delete pod -l operator.istio.io/component=EgressGateways -n istio-system --field-selector="$ready_node_selector"
fi

# kubectl delete blocks on a StatefulSet pod recreated under a name that returned 404
minikube kubectl -- delete pod --field-selector=metadata.namespace!=istio-system --all-namespaces --force --grace-period=0 --wait=false
minikube ssh -- sudo cat /etc/kubernetes/addons/storage-provisioner.yaml | minikube kubectl -- apply -f -
