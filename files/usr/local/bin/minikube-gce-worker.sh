#!/usr/bin/env -S bash -l

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

FIREWALL_TAG=minikube-gce-worker
INSTANCE_GROUP=minikube-gce-worker
ZONE=asia-northeast1-a
INTERFACE=wg-gce
# GCE's 1460-byte MTU less WireGuard's 60-byte IPv4 overhead
# cluster/bin/deploy-cilium.sh gives Cilium the same MTU
MTU=1400
# https://cloud.google.com/compute/docs/instances/spot#preemption-process
SHUTDOWN_GRACE_PERIOD=30s
SHUTDOWN_GRACE_PERIOD_CRITICAL_PODS=15s
# GCE reports RUNNING before the guest accepts SSH
BOOT_SECONDS=180

function usage() {
  cat <<EOS
Usage:
   minikube-gce-worker.sh

Peer, route and admit every running instance of the minikube-gce-worker and minikube-gce-worker-gpu instance groups over WireGuard, joining the ones that are not a node yet over IAP SSH with cluster/bin/join-worker.sh
Delete the node of every instance that is gone or was recreated after the node joined
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

project=$(gcloud config get-value project)
if [ -z "$project" ]; then
  exit 0
fi
instances=$(gcloud compute instances list --zones "$ZONE" --filter "name ~ ^${INSTANCE_GROUP}-" --format 'value(name,status,networkInterfaces[0].networkIP,networkInterfaces[0].accessConfigs[0].natIP,creationTimestamp.date("%s"),lastStartTimestamp.date("%s"),guestAccelerators[0].acceleratorType.basename())')
nodes=$(minikube kubectl -- get nodes -l '!node-role.kubernetes.io/control-plane' -o jsonpath='{range .items[*]}{.metadata.name}{" "}{.spec.providerID}{" "}{.metadata.creationTimestamp}{"\n"}{end}')

joined=()
while read -r name provider_id created; do
  if [ -z "$name" ]; then
    continue
  fi
  if [[ "$provider_id" != "gce://${project}/${ZONE}/"* ]]; then
    continue
  fi
  instance_created=$(awk -F '\t' -v name="$name" '$1 == name {print int($5)}' <<< "$instances")
  if [ -z "$instance_created" ] || [ "$(date -d "$created" +%s)" -lt "$instance_created" ]; then
    minikube kubectl -- delete node "$name"
    continue
  fi
  minikube kubectl -- label node "$name" node-role.kubernetes.io/worker= --overwrite
  joined+=("$name")
done <<< "$nodes"

if [ ! -d "/sys/class/net/${INTERFACE}" ]; then
  sudo ip link add "$INTERFACE" type wireguard
fi
if ! sudo test -f /etc/wireguard/minikube-gce-worker.key; then
  wg genkey | sudo install -D -m 600 /dev/stdin /etc/wireguard/minikube-gce-worker.key
fi
sudo wg set "$INTERFACE" private-key /etc/wireguard/minikube-gce-worker.key
sudo ip link set "$INTERFACE" mtu "$MTU" up
public_key=$(sudo wg show "$INTERFACE" public-key)
lan_interface=$(ip -4 -o route show default | awk 'NR == 1 {print $5}')
lan_cidr=$(ip -4 -o route show dev "$lan_interface" scope link proto kernel | awk 'NR == 1 {print $1}')
minikube_ip=$(minikube ip)

running=$(awk -F '\t' '$2 == "RUNNING" && $4 != ""' <<< "$instances")
names=()
worker_ips=()
external_ips=()
started=()
accelerators=()
while read -r name status ip external_ip created start accelerator; do
  if [ -z "$name" ]; then
    continue
  fi
  names+=("$name")
  worker_ips+=("$ip")
  external_ips+=("$external_ip")
  started+=("${start%.*}")
  accelerators+=("$accelerator")
done <<< "$running"

peers=$(sudo wg show "$INTERFACE" dump | awk 'NR > 1 {print $1, $3, $4}')
while read -r peer endpoint allowed_ip; do
  if [ -z "$peer" ]; then
    continue
  fi
  if ! awk -F '\t' -v ip="$allowed_ip" -v endpoint="$endpoint" '$3 "/32" == ip && $4 ":51820" == endpoint {found = 1} END {exit !found}' <<< "$running"; then
    sudo wg set "$INTERFACE" peer "$peer" remove
    sudo ip route del "$allowed_ip" dev "$INTERFACE"
  fi
done <<< "$peers"

admit_workers "$FIREWALL_TAG" "${worker_ips[@]}"

peers=$(sudo wg show "$INTERFACE" allowed-ips | awk '{print $2}')
token_file=$(mktemp "${XDG_RUNTIME_DIR:-/tmp}/minikube-gce-worker.XXXXXX")
trap 'rm -f "$token_file"' EXIT
for i in "${!names[@]}"; do
  if grep -qxF "${worker_ips[$i]}/32" <<< "$peers" && grep -qxF "${names[$i]}" <<< "$(printf '%s\n' "${joined[@]}")"; then
    continue
  fi

  echo "=== peering ${names[$i]} (${worker_ips[$i]})"
  # gcloud compute ssh prints ssh-keygen's output to stdout the first time it generates ~/.ssh/google_compute_engine
  if ! instance_public_key=$(gcloud compute ssh "${names[$i]}" --zone "$ZONE" --tunnel-through-iap --quiet --command "sudo bash -s" <<EOS | awk 'END {print}'
set -Eeo pipefail
# bash -s reads this script from stdin, which debconf would otherwise prompt on
export DEBIAN_FRONTEND=noninteractive
if ! command -v wg > /dev/null; then
  apt-get update -q > /dev/null
  apt-get install -yq wireguard-tools > /dev/null
fi
if [ ! -f /etc/wireguard/wg0.key ]; then
  (umask 077; wg genkey > /etc/wireguard/wg0.key)
fi
cat <<EOC > /etc/wireguard/wg0.conf
[Interface]
PrivateKey = \$(< /etc/wireguard/wg0.key)
# Cilium masquerades pod traffic only on a device holding an address: https://github.com/cilium/cilium/blob/v1.16.19/pkg/datapath/linux/devices_controller.go#L544-L546
Address = ${worker_ips[$i]}/32
ListenPort = 51820
MTU = ${MTU}

[Peer]
PublicKey = ${public_key}
AllowedIPs = ${minikube_ip%.*}.0/24, ${lan_cidr}
EOC
systemctl enable wg-quick@wg0
systemctl restart wg-quick@wg0
wg pubkey < /etc/wireguard/wg0.key
EOS
); then
    if [ "$(date +%s)" -lt "$((started[i] + BOOT_SECONDS))" ]; then
      echo "=== ${names[$i]} does not accept SSH yet"
      continue
    fi
    echo "${names[$i]} did not accept SSH within ${BOOT_SECONDS}s of starting" >&2
    exit 1
  fi
  sudo wg set "$INTERFACE" peer "$instance_public_key" endpoint "${external_ips[$i]}:51820" allowed-ips "${worker_ips[$i]}/32" persistent-keepalive 25
  sudo ip route replace "${worker_ips[$i]}/32" dev "$INTERFACE"

  if grep -qxF "${names[$i]}" <<< "$(printf '%s\n' "${joined[@]}")"; then
    continue
  fi

  collect_join_arguments "$token_file"
  accelerator_options=""
  if [ -n "${accelerators[$i]}" ]; then
    # The same taints and label the kube-env of terraform/google/main.tf announces to cluster-autoscaler
    accelerator_options=" --node-label cloud.google.com/gke-accelerator=${accelerators[$i]} --nvidia"
  fi

  echo "=== joining ${names[$i]} (${worker_ips[$i]})"
  gcloud compute scp /opt/hippocampus/cluster/bin/join-worker.sh ~/.docker/config.json "$token_file" "${names[$i]}:" --zone "$ZONE" --tunnel-through-iap --quiet
  gcloud compute ssh "${names[$i]}" --zone "$ZONE" --tunnel-through-iap --quiet --command "sudo bash join-worker.sh ${join_arguments[*]} --shutdown-grace-period ${SHUTDOWN_GRACE_PERIOD} --shutdown-grace-period-critical-pods ${SHUTDOWN_GRACE_PERIOD_CRITICAL_PODS} --remote-worker-value gce${accelerator_options} --provider-id gce://${project}/${ZONE}/${names[$i]}"
done
