#!/usr/bin/env bash

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

# The steps of cluster/bin/join-worker.sh and files/usr/local/bin/minikube-gce-worker.sh, and the images a joining node pulls first

# https://docs.cloud.google.com/compute/docs/metadata/querying-metadata
function attribute() {
  curl -fsSL --retry 5 --retry-all-errors -H "Metadata-Flavor: Google" "http://metadata.google.internal/computeMetadata/v1/instance/attributes/${1}"
}

kubernetes_version=$(attribute kubernetes-version)
containerd_version=$(attribute containerd-version)
runc_version=$(attribute runc-version)
nvidia=$(attribute nvidia)
images=$(attribute images)

export DEBIAN_FRONTEND=noninteractive
systemctl stop apt-daily.timer apt-daily-upgrade.timer apt-daily.service apt-daily-upgrade.service
apt-get update -q
apt-get install -yq ca-certificates curl gpg conntrack iptables socat ethtool avahi-daemon libnss-mdns wireguard-tools

curl -fsSL --retry 5 --retry-all-errors -o /tmp/containerd.tar.gz "https://github.com/containerd/containerd/releases/download/v${containerd_version}/containerd-${containerd_version}-linux-amd64.tar.gz"
tar zxf /tmp/containerd.tar.gz -C /usr/local
curl -fsSL --retry 5 --retry-all-errors -o /etc/systemd/system/containerd.service "https://raw.githubusercontent.com/containerd/containerd/v${containerd_version}/containerd.service"
curl -fsSL --retry 5 --retry-all-errors -o /usr/local/sbin/runc "https://github.com/opencontainers/runc/releases/download/v${runc_version}/runc.amd64"
chmod 755 /usr/local/sbin/runc
if [ "$nvidia" = "true" ]; then
  # DKMS builds the driver for every kernel installed later: https://github.com/dkms-project/dkms/blob/v3.0.11/kernel_postinst.d_dkms
  printf '%s hold\n' linux-gcp linux-image-gcp linux-headers-gcp | dpkg --set-selections
  # https://docs.nvidia.com/datacenter/tesla/driver-installation-guide/ubuntu.html
  apt-get install -yq "linux-headers-$(uname -r)"
  curl -fsSL --retry 5 --retry-all-errors -o /tmp/cuda-keyring.deb https://developer.download.nvidia.com/compute/cuda/repos/ubuntu2404/x86_64/cuda-keyring_1.1-1_all.deb
  dpkg -i /tmp/cuda-keyring.deb
  apt-get update -q
  apt-get install -yq libnvidia-compute nvidia-dkms-open nvidia-container-toolkit
fi

mkdir -p /etc/apt/keyrings
curl -fsSL --retry 5 --retry-all-errors -o /tmp/kubernetes.key "https://pkgs.k8s.io/core:/stable:/${kubernetes_version%.*}/deb/Release.key"
gpg --batch --yes --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg /tmp/kubernetes.key
echo "deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/${kubernetes_version%.*}/deb/ /" > /etc/apt/sources.list.d/kubernetes.list
apt-get update -q
apt-get install -yq "kubelet=${kubernetes_version#v}-1.1" "kubeadm=${kubernetes_version#v}-1.1"
apt-mark hold kubelet kubeadm

# terraform/google/wait-minikube-gce-worker-image-builder.sh sends the docker config files/usr/local/bin/minikube-gce-worker.sh gives join-worker.sh
until [ -f /run/registry-credentials.json ]; do
  sleep 5
done
registry_user=$(python3 -c 'import base64, json, sys; print(base64.b64decode(json.load(open(sys.argv[1]))["auths"]["ghcr.io"]["auth"]).decode())' /run/registry-credentials.json)
rm -f /run/registry-credentials.json
systemctl daemon-reload
systemctl start containerd
for image in $images; do
  options=()
  case "$image" in
    ghcr.io/*)
      options=(--user "$registry_user")
      ;;
  esac
  for attempt in 1 2 3 4 5; do
    if ctr --namespace k8s.io images pull "${options[@]}" "$image"; then
      break
    fi
    if [ "$attempt" -eq 5 ]; then
      exit 1
    fi
    sleep 5
  done
  # containerd's CRI resolves name:tag@digest by name@digest, a name ctr pull does not record
  name=${image%%@*}
  if [ "${name%:*}" != "$name" ]; then
    ctr --namespace k8s.io images tag "$image" "${name%:*}@${image#*@}"
  fi
done
systemctl stop containerd

[ "$(ls /lib/modules)" = "$(uname -r)" ]
apt-get clean
cloud-init clean --logs --machine-id
systemctl --no-block poweroff
