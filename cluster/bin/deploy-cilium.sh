#!/usr/bin/env bash

set -eo pipefail

CILIUM_VERSION=v0.14.2

t=$(mktemp -d)
curl -fsSL https://github.com/cilium/cilium-cli/releases/download/${CILIUM_VERSION}/cilium-linux-amd64.tar.gz | tar zx --no-same-owner -C $t cilium

$t/cilium install \
  --helm-set prometheus.enabled=true --helm-set operator.prometheus.enabled=true \
  --helm-set kubeProxyReplacement=strict --helm-set k8sServiceHost=$(minikube ip) --helm-set k8sServicePort=8443 \
  --helm-set socketLB.hostNamespaceOnly=true \
  --helm-set bpf.masquerade=true \
  # Enable BBR congestion control and IPv4 BIG-TCP for performance but it requires > 5.10 kernel
  #--helm-set bandwidthManager.enabled=true \
  #--helm-set bandwidthManager.bbr=true \
  #--helm-set ipv4.enabled=true --helm-set enableIPv4BIGTCP=true
$t/cilium hubble enable --relay --ui --helm-set hubble.metrics.enabled="{dns,drop,tcp,flow,flows-to-world,port-distribution,icmp,httpV2:exemplars=true;labelsContext=source_ip\,source_namespace\,source_workload\,destination_ip\,destination_namespace\,destination_workload\,traffic_direction}" --helm-set hubble.metrics.enableOpenMetrics=true

rm -rf $t
