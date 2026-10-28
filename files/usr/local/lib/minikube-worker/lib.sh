#!/usr/bin/env bash
# Sourced library (no set -e / trap of its own; the sourcing script owns those).

function admit_workers() {
  local tag="$1"
  shift
  local routes input_rules nat_rules desired admitted input_count input_handles nat_handles ip handle
  if [ "$#" -ne 0 ]; then
    routes=$(minikube ssh -- ip -4 route show | tr -d '\r')
  fi
  for ip in "$@"; do
    if ! awk -v ip="$ip" '$1 == ip && $2 == "via" && $3 == "192.168.39.1" {found = 1} END {exit !found}' <<< "$routes"; then
      minikube ssh -- sudo ip route replace "${ip}/32" via 192.168.39.1
    fi
  done

  input_rules=$(sudo nft -a list chain ip libvirt_network guest_input)
  nat_rules=$(sudo nft -a list chain ip libvirt_network guest_nat)
  desired=$(printf '%s\n' "$@" | LC_ALL=C sort)
  admitted=$(awk -v tag="$tag" 'index($0, "comment \"" tag "\"") {for (i = 1; i < NF; i++) if ($i == "daddr") print $(i + 1)}' <<< "$nat_rules" | LC_ALL=C sort)
  input_count=$(awk -v tag="$tag" 'index($0, "comment \"" tag "\"") {n++} END {print n + 0}' <<< "$input_rules")
  if [ "$admitted" = "$desired" ] && [ "$input_count" -eq "$(($# * 5))" ]; then
    return
  fi
  input_handles=$(awk -v tag="$tag" 'index($0, "comment \"" tag "\"") {print $NF}' <<< "$input_rules")
  nat_handles=$(awk -v tag="$tag" 'index($0, "comment \"" tag "\"") {print $NF}' <<< "$nat_rules")

  for ip in "$@"; do
    sudo nft insert rule ip libvirt_network guest_input ip saddr "$ip" oifname virbr1 tcp dport 8443 accept comment "\"${tag}\""
    sudo nft insert rule ip libvirt_network guest_input ip saddr "$ip" oifname virbr1 tcp dport 12379 accept comment "\"${tag}\""
    # https://docs.cilium.io/en/v1.16/operations/system_requirements/#firewall-rules
    sudo nft insert rule ip libvirt_network guest_input ip saddr "$ip" oifname virbr1 tcp dport 4240 accept comment "\"${tag}\""
    sudo nft insert rule ip libvirt_network guest_input ip saddr "$ip" oifname virbr1 udp dport 8472 accept comment "\"${tag}\""
    sudo nft insert rule ip libvirt_network guest_input ip saddr "$ip" oifname virbr1 meta l4proto icmp accept comment "\"${tag}\""
    sudo nft insert rule ip libvirt_network guest_nat ip saddr 192.168.39.0/24 ip daddr "$ip" return comment "\"${tag}\""
  done

  for handle in $input_handles; do
    sudo nft delete rule ip libvirt_network guest_input handle "$handle"
  done
  for handle in $nat_handles; do
    sudo nft delete rule ip libvirt_network guest_nat handle "$handle"
  done
}

function collect_join_arguments() {
  local token_file="$1"
  local control_plane_ip kubernetes_version container_runtime_version runc_version kubelet_configuration cgroup_driver join_command discovery_token_ca_cert_hash
  control_plane_ip=$(minikube ip)
  kubernetes_version=$(minikube kubectl -- get node minikube -o jsonpath='{.status.nodeInfo.kubeletVersion}')
  container_runtime_version=$(minikube kubectl -- get node minikube -o jsonpath='{.status.nodeInfo.containerRuntimeVersion}')
  runc_version=$(minikube ssh -- runc --version | tr -d '\r' | awk 'NR==1 {print $3}')
  kubelet_configuration=$(minikube kubectl -- get configmap kubelet-config -n kube-system -o jsonpath='{.data.kubelet}')
  cgroup_driver=$(awk '/^cgroupDriver:/ {print $2}' <<< "$kubelet_configuration")
  join_command=$(minikube ssh -- sudo "/var/lib/minikube/binaries/${kubernetes_version}/kubeadm" token create --print-join-command | tr -d '\r')
  awk '{for (i = 1; i < NF; i++) if ($i == "--token") print $(i + 1)}' <<< "$join_command" > "$token_file"
  discovery_token_ca_cert_hash=$(awk '{for (i = 1; i < NF; i++) if ($i == "--discovery-token-ca-cert-hash") print $(i + 1)}' <<< "$join_command")
  join_arguments=(--control-plane-ip "$control_plane_ip" --kubernetes-version "$kubernetes_version" --containerd-version "${container_runtime_version#containerd://}" --runc-version "$runc_version" --cgroup-driver "$cgroup_driver" --token-file "${token_file##*/}" --discovery-token-ca-cert-hash "$discovery_token_ca_cert_hash" --registry-credentials config.json)
}
