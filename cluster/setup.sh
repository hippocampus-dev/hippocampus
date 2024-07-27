#!/usr/bin/env bash

set -eo pipefail

. bin/export-secrets.sh

minikube ssh -- sudo bash /usr/local/bin/init.sh

# https://docs.cilium.io/en/stable/installation/taints/
kubectl taint nodes minikube node.cilium.io/agent-not-ready=true:NoExecute

kubectl label namespace/default name=default
kubectl label namespace/kube-node-lease name=kube-node-lease
kubectl label namespace/kube-public name=kube-public
kubectl label namespace/kube-system name=kube-system

bash bin/deploy-cilium.sh

minikube ssh -- sudo cat /etc/kubernetes/addons/storage-provisioner.yaml | kubectl apply -f -

bash bin/deploy-istio.sh

bash bin/deploy-vault.sh
bash bin/setup-vault.sh

kubectl wait --for=condition=Ready pod -l istio.io/rev=default -n istio-system --timeout=30m

bash bin/deploy-argocd.sh

# https://github.com/kubernetes/minikube/issues/18021
images=("ghcr.io/kaidotio/hippocampus/ephemeral-container:main" "ghcr.io/kaidotio/hippocampus/singleuser-notebook:main")
for image in "${images[@]}"; do
  t=$(mktemp)
  docker pull $image
  docker image save -o $t $image
  minikube image load $t
done
