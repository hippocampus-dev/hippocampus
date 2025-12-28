#!/usr/bin/env bash

set -e

ENTRYPOINT=$(cd $(dirname ${BASH_SOURCE[0]}); pwd)

kubectl apply -k ${ENTRYPOINT}/../manifests/cilium-etcd/overlays/dev
until kubectl -n kube-system wait --for=condition=Ready pod -l app.kubernetes.io/name=cilium --timeout=10m; do sleep 1; done

CILIUM_CLI_VERSION=v0.16.19

t=$(mktemp -d)
curl -fsSL https://github.com/cilium/cilium-cli/releases/download/${CILIUM_CLI_VERSION}/cilium-linux-amd64.tar.gz | tar zx --no-same-owner -C ${t} cilium

opt="--version 1.16.19"
# Enable metrics
opt=" --helm-set prometheus.enabled=true --helm-set operator.prometheus.enabled=true"
# https://docs.cilium.io/en/latest/installation/k8s-install-external-etcd/
opt+=" --helm-set etcd.enabled=true --helm-set etcd.endpoints[0]=http://127.0.0.1:12379"
# https://docs.cilium.io/en/stable/network/kubernetes/kubeproxy-free/
opt+=" --helm-set kubeProxyReplacement=true --helm-set k8sServiceHost=$(minikube ip) --helm-set k8sServicePort=8443"
# https://docs.cilium.io/en/latest/network/servicemesh/istio/
opt+=" --helm-set socketLB.hostNamespaceOnly=true --helm-set cni.exclusive=false"
# Enable eBPF-based masquerading for performance but it requires > 4.19 kernel
opt+=" --helm-set bpf.masquerade=true"
# Enable BBR congestion control and IPv4 BIG-TCP for performance but it requires > 5.10 kernel
#opt+=" --helm-set bandwidthManager.enabled=true --helm-set bandwidthManager.bbr=true"
#opt+=" --helm-set ipv4.enabled=true --helm-set enableIPv4BIGTCP=true"

${t}/cilium install ${opt} --helm-set hubble.metrics.enabled="{dns,drop,tcp,flow,flows-to-world,port-distribution,icmp,httpV2:exemplars=true;labelsContext=source_ip\,source_namespace\,source_workload\,destination_ip\,destination_namespace\,destination_workload\,traffic_direction}" --helm-set hubble.metrics.enableOpenMetrics=true --set hubble.enabled=true --set hubble.export.dynamic.enabled=true --set hubble.export.dynamic.config.content[0].name=allowlist --set hubble.export.dynamic.config.content[0].filePath=/var/run/cilium/hubble/events.log --set hubble.export.dynamic.config.content[0].includeFilters[0].source_label[0]="k8s:hubble.cilium.io/export.source=true" --set hubble.export.dynamic.config.content[0].includeFilters[1].destination_label[0]="k8s:hubble.cilium.io/export.destination=true"
${t}/cilium hubble enable --relay --ui

rm -rf ${t}
