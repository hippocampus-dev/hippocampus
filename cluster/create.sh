#!/usr/bin/env bash

set -e

export ISTIO_VERSION=1.23.2

ENTRYPOINT=$(cd $(dirname ${BASH_SOURCE[0]}); pwd)

. ${ENTRYPOINT}/bin/export.sh

minikube ssh -- sudo bash /usr/local/bin/init.sh

kubectl label namespace/default name=default
kubectl label namespace/kube-node-lease name=kube-node-lease
kubectl label namespace/kube-public name=kube-public
kubectl label namespace/kube-system name=kube-system

bash ${ENTRYPOINT}/bin/deploy-cilium.sh

minikube ssh -- sudo cat /etc/kubernetes/addons/storage-provisioner.yaml | kubectl apply -f -

bash ${ENTRYPOINT}/bin/deploy-istio.sh

bash ${ENTRYPOINT}/bin/deploy-vault.sh
bash ${ENTRYPOINT}/bin/initialize-vault.sh

sleep 1
kubectl wait --for=condition=Ready pod -l istio.io/rev=default -n istio-system --timeout=30m

kubectl create namespace assets
kubectl create configmap metadata --from-literal=ISTIO_VERSION=${ISTIO_VERSION} -n assets

bash ${ENTRYPOINT}/bin/deploy-argocd.sh

# https://github.com/kubernetes/minikube/issues/18021
images=("ghcr.io/hippocampus-dev/hippocampus/ephemeral-container:main" "ghcr.io/hippocampus-dev/hippocampus/singleuser-notebook:main")
for image in "${images[@]}"; do
  t=$(mktemp)
  docker pull ${image}
  docker image save -o ${t} ${image}
  minikube image load ${t}
  rm ${t}
done
