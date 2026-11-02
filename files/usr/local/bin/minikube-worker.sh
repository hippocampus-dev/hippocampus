#!/usr/bin/env -S bash -l

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

FIREWALL_TAG=minikube-worker
# google_compute_subnetwork.minikube_gce_worker in terraform/google/main.tf
GCE_WORKER_CIDR=192.168.40.0/24
SHUTDOWN_GRACE_PERIOD=120s
SHUTDOWN_GRACE_PERIOD_CRITICAL_PODS=30s
# VM_NAME in files/usr/local/bin/minikube-add-hyperv-worker.sh
WORKERS=(
  ubuntu@hyperv-worker.local
)

function usage() {
  cat <<EOS
Usage:
   minikube-worker.sh

Route and admit every joined worker between the LAN and mk-minikube, resolving each node's <name>.local over mDNS
Join each of ${WORKERS[*]} that accepts SSH and is not a node yet with cluster/bin/join-worker.sh
A retired worker needs kubectl delete node to lose its access, and a host that has joined once is never joined again until it is reinstalled
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
  usage >&2
  exit 1
fi

source /usr/local/lib/minikube-worker/lib.sh

nodes=$(minikube kubectl -- get nodes -l '!node-role.kubernetes.io/control-plane' -o jsonpath='{range .items[*]}{.metadata.name}{" "}{.spec.providerID}{"\n"}{end}')
worker_ips=()
while read -r name provider_id; do
  if [ -z "$name" ]; then
    continue
  fi
  # minikube-gce-worker.sh admits these
  if [[ "$provider_id" == gce://* ]]; then
    continue
  fi
  minikube kubectl -- label node "$name" node-role.kubernetes.io/worker= --overwrite
  if ! resolved=$(getent ahostsv4 "${name}.local"); then
    echo "Skipping unresolvable worker ${name}.local" >&2
    continue
  fi
  ip=$(awk 'NR == 1 {print $1}' <<< "$resolved")
  worker_ips+=("$ip")
done <<< "$nodes"

join_targets=()
join_ips=()
for target in "${WORKERS[@]}"; do
  host="${target#*@}"
  if awk -v name="${host%.local}" '$1 == name {found = 1} END {exit !found}' <<< "$nodes"; then
    continue
  fi
  # kubeadm join's FileAvailable--etc-kubernetes-kubelet.conf preflight check rejects a host that has joined once
  if ! ssh -o BatchMode=yes -o ConnectTimeout=5 "$target" test ! -f /etc/kubernetes/kubelet.conf 2> /dev/null; then
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
