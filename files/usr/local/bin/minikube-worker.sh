#!/usr/bin/env -S bash -l

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

readonly FIREWALL_TAG=minikube-worker
# google_compute_subnetwork.minikube_gce_worker in terraform/google/main.tf
readonly GCE_WORKER_CIDR=192.168.40.0/24
readonly SHUTDOWN_GRACE_PERIOD=120s
readonly SHUTDOWN_GRACE_PERIOD_CRITICAL_PODS=30s
readonly WORKERS=()

usage() {
  cat <<EOS
Usage:
   minikube-worker.sh

Route and admit every joined worker between the LAN and mk-minikube, resolving each node's <name>.local over mDNS
Join each worker of WORKERS and HYPERV_WORKERS that accepts SSH and is not a node yet with cluster/bin/join-worker.sh
Admit each of them that has joined but is not a node, pointing its control-plane.minikube.internal at the current minikube ip and restarting its kubelet so it registers again
Annotate each node of HYPERV_WORKERS lacking it with the MAC address files/usr/local/bin/minikube-start.sh wakes its Hyper-V host on
Relocate to each uncordoned, Ready node of HYPERV_WORKERS every pod on the control plane node annotated with relocatable-volume-controller.kaidotio.github.io/volumes
A retired worker needs kubectl delete node and removal from WORKERS or HYPERV_WORKERS to lose its access, and a host that has joined once is never joined again until it is reinstalled
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

if [ "${#args[@]}" -ne 0 ]; then
  usage 1>&2
  exit 1
fi

source /usr/local/lib/minikube-worker/lib.sh
source /usr/local/lib/hyperv-host/lib.sh

nodes=$(retry minikube kubectl -- get nodes -l '!node-role.kubernetes.io/control-plane' -o jsonpath='{range .items[*]}{.metadata.name}{" "}{.spec.providerID}{"\n"}{end}')
worker_ips=()
while read -r name provider_id; do
  if [ -z "$name" ]; then
    continue
  fi
  # minikube-gce-worker.sh admits these
  case "$provider_id" in
    gce://*)
      continue
      ;;
  esac
  minikube kubectl -- label node "$name" node-role.kubernetes.io/worker= --overwrite
  if ! resolved=$(getent ahostsv4 "${name}.local"); then
    echo "Skipping unresolvable worker ${name}.local" 1>&2
    continue
  fi
  ip=$(awk 'NR == 1 {print $1}' <<< "$resolved")
  worker_ips+=("$ip")
done <<< "$nodes"

control_plane_ip=$(minikube ip)
join_targets=()
join_ips=()
for worker in "${WORKERS[@]}" "${HYPERV_WORKERS[@]}"; do
  read -r target hyperv_host hyperv_switch _ <<< "$worker"
  host="${target#*@}"
  if awk -v name="${host%.local}" '$1 == name {found = 1} END {exit !found}' <<< "$nodes"; then
    if [ -z "$hyperv_host" ]; then
      continue
    fi
    mac=$(minikube kubectl -- get node "${host%.local}" -o jsonpath='{.metadata.annotations.wake-on-lan\.kaidotio\.github\.io/mac}')
    if [ -z "$mac" ] && output=$(ssh -o BatchMode=yes -o ConnectTimeout=5 -o ServerAliveInterval=15 -o ServerAliveCountMax=4 "$hyperv_host" "chcp 65001 >NUL && powershell -NoProfile -NonInteractive -Command \"(Get-NetAdapter -InterfaceDescription (Get-VMSwitch -Name '${hyperv_switch}').NetAdapterInterfaceDescription).MacAddress\""); then
      mac=$(tr -d '\r' <<< "$output" | awk 'NF {line = $0} END {gsub("-", ":", line); print tolower(line)}')
      minikube kubectl -- annotate node "${host%.local}" "wake-on-lan.kaidotio.github.io/mac=${mac}" --overwrite
    fi
    continue
  fi
  # kubeadm join's FileAvailable--etc-kubernetes-kubelet.conf preflight check rejects a host that has joined once
  if ! ssh -o BatchMode=yes -o ConnectTimeout=5 "$target" test ! -f /etc/kubernetes/kubelet.conf 2> /dev/null; then
    # minikube delete gives the recreated VM a different address from the one join-worker.sh wrote
    # The kubelet registers its node only when it starts: https://github.com/kubernetes/kubernetes/issues/71398
    ssh -o BatchMode=yes -o ConnectTimeout=5 "$target" "{ grep -qxF '${control_plane_ip} control-plane.minikube.internal' /etc/hosts || { sudo sed -i '/ control-plane\.minikube\.internal\$/d' /etc/hosts && echo '${control_plane_ip} control-plane.minikube.internal' | sudo tee -a /etc/hosts > /dev/null; }; } && sudo systemctl restart kubelet" 2> /dev/null || true
    if resolved=$(getent ahostsv4 "$host"); then
      ip=$(awk 'NR == 1 {print $1}' <<< "$resolved")
      worker_ips+=("$ip")
    fi
    continue
  fi
  resolved=$(getent ahostsv4 "$host")
  ip=$(awk 'NR == 1 {print $1}' <<< "$resolved")
  worker_ips+=("$ip")
  join_targets+=("$target")
  join_ips+=("$ip")
done

admit_workers "$FIREWALL_TAG" "${worker_ips[@]}"

token_file=$(mktemp "${XDG_RUNTIME_DIR:-/tmp}/minikube-worker.XXXXXX")
trap 'rm -f "$token_file"' EXIT
for i in "${!join_targets[@]}"; do
  collect_join_arguments "$token_file"

  echo "=== joining ${join_targets[$i]} (${join_ips[$i]})"
  scp -o BatchMode=yes -o ServerAliveInterval=15 -o ServerAliveCountMax=4 /opt/hippocampus/cluster/bin/join-worker.sh ~/.docker/config.json "$token_file" "${join_targets[$i]}:"
  ssh -o BatchMode=yes -o ServerAliveInterval=15 -o ServerAliveCountMax=4 "${join_targets[$i]}" sudo bash join-worker.sh "${join_arguments[@]}" --gateway "$(uname -n).local" --shutdown-grace-period "$SHUTDOWN_GRACE_PERIOD" --shutdown-grace-period-critical-pods "$SHUTDOWN_GRACE_PERIOD_CRITICAL_PODS" --remote-worker-value lan --node-label node.kaidotio.github.io/remote-worker=lan --route-cidr "$GCE_WORKER_CIDR"
done

for worker in "${HYPERV_WORKERS[@]}"; do
  read -r target _ <<< "$worker"
  host="${target#*@}"
  node=$(minikube kubectl -- get node "${host%.local}" --ignore-not-found -o json)
  if [ -z "$node" ]; then
    continue
  fi
  ready=$(jq -r 'select(.spec.unschedulable != true) | .status.conditions[] | select(.type == "Ready" and .status == "True") | .status' <<< "$node")
  if [ -z "$ready" ]; then
    continue
  fi
  control_plane=$(minikube kubectl -- get nodes -l node-role.kubernetes.io/control-plane -o jsonpath='{.items[0].metadata.name}')
  pods=$(minikube kubectl -- get pods -A -o json | jq -r --arg node "$control_plane" --arg key "$RELOCATABLE_ANNOTATION" '.items[] | select(.metadata.annotations[$key] != null and .spec.nodeName == $node) | .metadata.name')
  if [ -n "$pods" ]; then
    relocate_volumes "$control_plane" "${host%.local}"
  fi
done
