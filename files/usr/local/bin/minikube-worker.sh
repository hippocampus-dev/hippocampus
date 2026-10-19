#!/usr/bin/env -S bash -l

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

FIREWALL_TAG=minikube-worker

function usage() {
  cat <<EOS
Usage:
   minikube-worker.sh [[user@]host...]

Route and admit every joined worker between the LAN and mk-minikube, resolving each node's <name>.local over mDNS
Join each host given that is not a node yet over SSH with cluster/bin/join-worker.sh
A retired worker needs kubectl delete node to lose its access
EOS
}

args=()
while (( $# )); do
  case "$1" in
    -h|--help)
      usage
      exit 0
      ;;
    --)
      shift
      break
      ;;
    -*|--*)
      echo "Unsupported flag $1" 1>&2
      exit 1
      ;;
    *)
      args+=("$1")
      shift
      ;;
  esac
done

nodes=$(minikube kubectl -- get nodes -l '!node-role.kubernetes.io/control-plane' -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}')
worker_ips=()
while read -r name; do
  if [ -z "$name" ]; then
    continue
  fi
  if ! resolved=$(getent ahostsv4 "${name}.local"); then
    echo "Skipping unresolvable worker ${name}.local" >&2
    continue
  fi
  ip=$(awk 'NR == 1 {print $1}' <<< "$resolved")
  worker_ips+=("$ip")
done <<< "$nodes"

join_targets=()
join_ips=()
for target in "${args[@]}"; do
  resolved=$(getent ahostsv4 "${target#*@}")
  ip=$(awk 'NR == 1 {print $1}' <<< "$resolved")
  if grep -qxF "$ip" <<< "$(printf '%s\n' "${worker_ips[@]}")"; then
    continue
  fi
  worker_ips+=("$ip")
  join_targets+=("$target")
  join_ips+=("$ip")
done

input_handles=$(sudo nft -a list chain ip libvirt_network guest_input | awk -v tag="$FIREWALL_TAG" 'index($0, "comment \"" tag "\"") {print $NF}')
nat_handles=$(sudo nft -a list chain ip libvirt_network guest_nat | awk -v tag="$FIREWALL_TAG" 'index($0, "comment \"" tag "\"") {print $NF}')

for ip in "${worker_ips[@]}"; do
  minikube ssh -- sudo ip route replace "${ip}/32" via 192.168.39.1
  sudo nft insert rule ip libvirt_network guest_input ip saddr "$ip" oifname virbr1 tcp dport 8443 accept comment "\"${FIREWALL_TAG}\""
  sudo nft insert rule ip libvirt_network guest_input ip saddr "$ip" oifname virbr1 tcp dport 12379 accept comment "\"${FIREWALL_TAG}\""
  # https://docs.cilium.io/en/v1.16/operations/system_requirements/#firewall-rules
  sudo nft insert rule ip libvirt_network guest_input ip saddr "$ip" oifname virbr1 tcp dport 4240 accept comment "\"${FIREWALL_TAG}\""
  sudo nft insert rule ip libvirt_network guest_input ip saddr "$ip" oifname virbr1 udp dport 8472 accept comment "\"${FIREWALL_TAG}\""
  sudo nft insert rule ip libvirt_network guest_input ip saddr "$ip" oifname virbr1 meta l4proto icmp accept comment "\"${FIREWALL_TAG}\""
  sudo nft insert rule ip libvirt_network guest_nat ip saddr 192.168.39.0/24 ip daddr "$ip" return comment "\"${FIREWALL_TAG}\""
done

for handle in $input_handles; do
  sudo nft delete rule ip libvirt_network guest_input handle "$handle"
done
for handle in $nat_handles; do
  sudo nft delete rule ip libvirt_network guest_nat handle "$handle"
done

for i in "${!join_targets[@]}"; do
  kubernetes_version=$(minikube kubectl -- get node minikube -o jsonpath='{.status.nodeInfo.kubeletVersion}')
  container_runtime_version=$(minikube kubectl -- get node minikube -o jsonpath='{.status.nodeInfo.containerRuntimeVersion}')
  runc_version=$(minikube ssh -- runc --version | tr -d '\r' | awk 'NR==1 {print $3}')
  kubelet_configuration=$(minikube kubectl -- get configmap kubelet-config -n kube-system -o jsonpath='{.data.kubelet}')
  cgroup_driver=$(awk '/^cgroupDriver:/ {print $2}' <<< "$kubelet_configuration")
  join_command=$(minikube ssh -- sudo "/var/lib/minikube/binaries/${kubernetes_version}/kubeadm" token create --print-join-command | tr -d '\r')
  token_file=$(mktemp "${XDG_RUNTIME_DIR:-/tmp}/minikube-worker.XXXXXX")
  trap 'rm -f "$token_file"' EXIT
  awk '{for (i = 1; i < NF; i++) if ($i == "--token") print $(i + 1)}' <<< "$join_command" > "$token_file"
  discovery_token_ca_cert_hash=$(awk '{for (i = 1; i < NF; i++) if ($i == "--discovery-token-ca-cert-hash") print $(i + 1)}' <<< "$join_command")

  echo "=== joining ${join_targets[$i]} (${join_ips[$i]})"
  scp /opt/hippocampus/cluster/bin/join-worker.sh ~/.docker/config.json "$token_file" "${join_targets[$i]}:"
  ssh -t "${join_targets[$i]}" sudo bash join-worker.sh --control-plane-ip "$(minikube ip)" --gateway "$(uname -n).local" --kubernetes-version "$kubernetes_version" --containerd-version "${container_runtime_version#containerd://}" --runc-version "$runc_version" --cgroup-driver "$cgroup_driver" --token-file "${token_file##*/}" --discovery-token-ca-cert-hash "$discovery_token_ca_cert_hash" --registry-credentials config.json
done
