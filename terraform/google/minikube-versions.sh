#!/usr/bin/env bash

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

# The versions files/usr/local/lib/minikube-worker/lib.sh reads from minikube for cluster/bin/join-worker.sh
kubernetes_version=$(minikube kubectl -- get node minikube -o jsonpath='{.status.nodeInfo.kubeletVersion}')
container_runtime_version=$(minikube kubectl -- get node minikube -o jsonpath='{.status.nodeInfo.containerRuntimeVersion}')
runc_version=$(minikube ssh -- runc --version | tr -d '\r' | awk 'NR == 1 {print $3}')
# A joining node pulls these before it can take other pods
cilium_image=$(minikube kubectl -- get daemonset cilium -n kube-system -o jsonpath='{.spec.template.spec.containers[0].image}')
nvidia_device_plugin_image=$(minikube kubectl -- get daemonset nvidia-device-plugin -n kube-system -o jsonpath='{.spec.template.spec.containers[0].image}')
jq -n --arg kubernetes_version "$kubernetes_version" --arg containerd_version "${container_runtime_version#containerd://}" --arg runc_version "$runc_version" --arg cilium_image "$cilium_image" --arg nvidia_device_plugin_image "$nvidia_device_plugin_image" '{$kubernetes_version, $containerd_version, $runc_version, $cilium_image, $nvidia_device_plugin_image}'
