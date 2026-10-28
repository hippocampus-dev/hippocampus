#!/usr/bin/env bash

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

function usage() {
  cat <<EOS
Usage:
   join-worker.sh --control-plane-ip <ip> [--gateway <host> [--route-cidr <cidr>...]] --kubernetes-version <version> --containerd-version <version> --runc-version <version> --cgroup-driver <driver> --token-file <path> --discovery-token-ca-cert-hash <hash> --registry-credentials <path> --shutdown-grace-period <duration> --shutdown-grace-period-critical-pods <duration> --remote-worker-value <value> [--provider-id <id>] [--node-label <key=value>...] [--nvidia]

Join this Ubuntu 24.04 host to the minikube cluster as a tainted worker; files/usr/local/bin/minikube-worker.sh and files/usr/local/bin/minikube-gce-worker.sh run it with the arguments
Without --gateway the host must already route to the control plane
--nvidia installs the NVIDIA driver, registers nvidia-container-runtime as the containerd runtime the nvidia RuntimeClass names and taints the node with nvidia.com/gpu
EOS
}

route_cidrs=()
node_labels=()
while (( $# )); do
  case "$1" in
    -h|--help)
      usage
      exit 0
      ;;
    --control-plane-ip)
      control_plane_ip="$2"
      shift 2
      ;;
    --gateway)
      gateway="$2"
      shift 2
      ;;
    --route-cidr)
      route_cidrs+=("$2")
      shift 2
      ;;
    --kubernetes-version)
      kubernetes_version="$2"
      shift 2
      ;;
    --containerd-version)
      containerd_version="$2"
      shift 2
      ;;
    --runc-version)
      runc_version="$2"
      shift 2
      ;;
    --cgroup-driver)
      cgroup_driver="$2"
      shift 2
      ;;
    --token-file)
      token_file="$2"
      shift 2
      ;;
    --discovery-token-ca-cert-hash)
      discovery_token_ca_cert_hash="$2"
      shift 2
      ;;
    --registry-credentials)
      registry_credentials="$2"
      shift 2
      ;;
    --shutdown-grace-period)
      shutdown_grace_period="$2"
      shift 2
      ;;
    --shutdown-grace-period-critical-pods)
      shutdown_grace_period_critical_pods="$2"
      shift 2
      ;;
    --provider-id)
      provider_id="$2"
      shift 2
      ;;
    --remote-worker-value)
      remote_worker_value="$2"
      shift 2
      ;;
    --node-label)
      node_labels+=("$2")
      shift 2
      ;;
    --nvidia)
      nvidia=true
      shift
      ;;
    *)
      echo "Unsupported flag $1" 1>&2
      exit 1
      ;;
  esac
done

systemd_cgroup=false
if [ "$cgroup_driver" = "systemd" ]; then
  systemd_cgroup=true
fi

printf 'overlay\nbr_netfilter\n' > /etc/modules-load.d/kubernetes.conf
modprobe overlay
modprobe br_netfilter
# fs.inotify.max_user_instances follows files/home/kai/.minikube/files/usr/local/bin/init.sh
# kernel.dmesg_restrict follows the minikube kernel for cluster/manifests/node-problem-detector/base/daemon_set.yaml
printf 'net.ipv4.ip_forward = 1\nnet.bridge.bridge-nf-call-iptables = 1\nfs.inotify.max_user_instances = 1024\nkernel.dmesg_restrict = 0\n' > /etc/sysctl.d/kubernetes.conf
sysctl --system > /dev/null

apt-get update -q
apt-get install -yq ca-certificates curl gpg conntrack iptables socat ethtool avahi-daemon libnss-mdns

# terraform/google/minikube-gce-worker-image.sh bakes these into the GCE worker images at the versions minikube has
if [ "$(/usr/local/bin/containerd --version | awk '{print $3}')" != "v${containerd_version}" ]; then
  curl -fsSL --retry 5 --retry-all-errors -o /tmp/containerd.tar.gz "https://github.com/containerd/containerd/releases/download/v${containerd_version}/containerd-${containerd_version}-linux-amd64.tar.gz"
  tar zxf /tmp/containerd.tar.gz -C /usr/local
  curl -fsSL --retry 5 --retry-all-errors -o /etc/systemd/system/containerd.service "https://raw.githubusercontent.com/containerd/containerd/v${containerd_version}/containerd.service"
fi
if [ "$(/usr/local/sbin/runc --version | awk 'NR == 1 {print $3}')" != "$runc_version" ]; then
  curl -fsSL --retry 5 --retry-all-errors -o /usr/local/sbin/runc "https://github.com/opencontainers/runc/releases/download/v${runc_version}/runc.amd64"
  chmod 755 /usr/local/sbin/runc
fi
mkdir -p /etc/containerd
containerd config default > /etc/containerd/config.toml
sed -i "s/SystemdCgroup = false/SystemdCgroup = ${systemd_cgroup}/" /etc/containerd/config.toml
grep -q "SystemdCgroup = ${systemd_cgroup}" /etc/containerd/config.toml
# containers inherit containerd's RLIMIT_NOFILE, which upstream containerd.service leaves at systemd's 1024 soft limit while minikube's sets this: https://github.com/kubernetes/minikube/blob/v1.38.1/deploy/iso/minikube-iso/arch/x86_64/package/containerd-bin/containerd.service#L21
mkdir -p /etc/systemd/system/containerd.service.d
printf '[Service]\nLimitNOFILE=1048576\n' > /etc/systemd/system/containerd.service.d/limit-nofile.conf
if [ "$nvidia" = "true" ]; then
  # https://docs.nvidia.com/datacenter/tesla/driver-installation-guide/ubuntu.html
  apt-get install -yq "linux-headers-$(uname -r)"
  curl -fsSL --retry 5 --retry-all-errors -o /tmp/cuda-keyring.deb https://developer.download.nvidia.com/compute/cuda/repos/ubuntu2404/x86_64/cuda-keyring_1.1-1_all.deb
  dpkg -i /tmp/cuda-keyring.deb
  apt-get update -q
  apt-get install -yq --no-upgrade libnvidia-compute nvidia-dkms-open nvidia-container-toolkit
  modprobe nvidia
  nvidia-ctk runtime configure --runtime=containerd
fi
systemctl daemon-reload
systemctl enable --now containerd

mkdir -p /etc/apt/keyrings
curl -fsSL --retry 5 --retry-all-errors -o /tmp/kubernetes.key "https://pkgs.k8s.io/core:/stable:/${kubernetes_version%.*}/deb/Release.key"
gpg --batch --yes --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg /tmp/kubernetes.key
echo "deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/${kubernetes_version%.*}/deb/ /" > /etc/apt/sources.list.d/kubernetes.list
apt-get update -q
apt-get install -yq --allow-change-held-packages --allow-downgrades "kubelet=${kubernetes_version#v}-1.1" "kubeadm=${kubernetes_version#v}-1.1"
apt-mark hold kubelet kubeadm
systemctl enable kubelet

if [ -n "$gateway" ]; then
  cat <<EOS > /usr/local/sbin/minikube-route
#!/usr/bin/env bash

set -Eeo pipefail
trap 'echo "exit \$?: \$BASH_COMMAND(line \$LINENO)" >&2' ERR

resolved=\$(getent ahostsv4 ${gateway})
gateway_ip=\$(awk 'NR == 1 {print \$1}' <<< "\$resolved")
for cidr in ${control_plane_ip%.*}.0/24 ${route_cidrs[*]}; do
  ip route replace "\$cidr" via "\$gateway_ip"
done
EOS
  chmod 755 /usr/local/sbin/minikube-route

  cat <<EOS > /etc/systemd/system/minikube-route.service
[Unit]
Wants=network-online.target avahi-daemon.service
After=network-online.target avahi-daemon.service
Before=kubelet.service

[Service]
Type=oneshot
ExecStart=/usr/local/sbin/minikube-route

[Install]
WantedBy=multi-user.target
EOS
  cat <<EOS > /etc/systemd/system/minikube-route.timer
[Timer]
OnCalendar=minutely
Persistent=true

[Install]
WantedBy=timers.target
EOS
  systemctl daemon-reload
  systemctl enable minikube-route.service minikube-route.timer
  systemctl start minikube-route.service minikube-route.timer
fi

sed -i '/ control-plane\.minikube\.internal$/d' /etc/hosts
echo "${control_plane_ip} control-plane.minikube.internal" >> /etc/hosts

# minikube's kubelet-config sets authentication.x509.clientCAFile to this path and kubeadm join keeps it: https://github.com/kubernetes/kubernetes/blob/v1.37.1/cmd/kubeadm/app/componentconfigs/kubelet.go#L146-L150
mkdir -p /var/lib/minikube/certs
ln -sf /etc/kubernetes/pki/ca.crt /var/lib/minikube/certs/ca.crt

install -D -m 600 "$registry_credentials" /var/lib/kubelet/config.json

nvidia_taints=""
if [ "$nvidia" = "true" ]; then
  nvidia_taints="    - key: nvidia.com/gpu"$'\n'"      effect: NoSchedule"$'\n'
fi

kubelet_extra_args="  kubeletExtraArgs:"$'\n'
# The kubelet-config ConfigMap that minikube writes sets imageGCHighThresholdPercent: 100 and zero evictionHard
# The kubelet options of files/usr/local/bin/minikube-start.sh reach only the control plane's kubelet
kubelet_extra_args+="    - name: image-gc-high-threshold"$'\n'"      value: \"80\""$'\n'
kubelet_extra_args+="    - name: image-gc-low-threshold"$'\n'"      value: \"70\""$'\n'
kubelet_extra_args+="    - name: eviction-soft"$'\n'"      value: \"memory.available<100Mi\""$'\n'
kubelet_extra_args+="    - name: eviction-soft-grace-period"$'\n'"      value: \"memory.available=1m\""$'\n'
kubelet_extra_args+="    - name: eviction-hard"$'\n'"      value: \"memory.available<100Mi,nodefs.available<5%,imagefs.available<5%\""$'\n'
kubelet_extra_args+="    - name: max-pods"$'\n'"      value: \"500\""$'\n'
kubelet_extra_args+="    - name: housekeeping-interval"$'\n'"      value: \"15s\""$'\n'
kubelet_extra_args+="    - name: serialize-image-pulls"$'\n'"      value: \"false\""$'\n'
kubelet_extra_args+="    - name: container-log-max-size"$'\n'"      value: \"1Mi\""$'\n'
kubelet_extra_args+="    - name: container-log-max-files"$'\n'"      value: \"2\""$'\n'
kubelet_extra_args+="    - name: system-reserved"$'\n'"      value: \"cpu=500m,memory=1Gi,ephemeral-storage=10Gi,pid=1000\""$'\n'
kubelet_extra_args+="    - name: feature-gates"$'\n'"      value: \"HPAScaleToZero=true,MutatingAdmissionPolicy=true,ResourceHealthStatus=true,ReduceDefaultCrashLoopBackOffDecay=true,CRDObservedGenerationTracking=true,GenericWorkload=true,StatefulSetRecreateStrategy=true,InPlacePodVerticalScalingMemoryBackedVolumes=true,EmptyDirVolumeMode=true\""$'\n'
if [ -n "$provider_id" ]; then
  kubelet_extra_args+="    - name: provider-id"$'\n'"      value: ${provider_id}"$'\n'
fi
if [ "${#node_labels[@]}" -ne 0 ]; then
  kubelet_extra_args+="    - name: node-labels"$'\n'"      value: $(IFS=,; echo "${node_labels[*]}")"$'\n'
fi

join_configuration=$(mktemp)
trap 'rm -f "$join_configuration"' EXIT
cat <<EOS > "$join_configuration"
apiVersion: kubeadm.k8s.io/v1beta4
kind: JoinConfiguration
discovery:
  bootstrapToken:
    apiServerEndpoint: control-plane.minikube.internal:8443
    token: $(< "$token_file")
    caCertHashes:
      - ${discovery_token_ca_cert_hash}
nodeRegistration:
  taints:
    - key: node.kaidotio.github.io/remote-worker
      value: "${remote_worker_value}"
      effect: NoSchedule
    # https://docs.cilium.io/en/stable/installation/taints/
    - key: node.cilium.io/agent-not-ready
      value: "true"
      effect: NoExecute
${nvidia_taints}${kubelet_extra_args}
EOS
kubeadm join --config "$join_configuration"

sed -i "s/^shutdownGracePeriod: .*/shutdownGracePeriod: ${shutdown_grace_period}/" /var/lib/kubelet/config.yaml
sed -i "s/^shutdownGracePeriodCriticalPods: .*/shutdownGracePeriodCriticalPods: ${shutdown_grace_period_critical_pods}/" /var/lib/kubelet/config.yaml
if grep -q '^maxParallelImagePulls:' /var/lib/kubelet/config.yaml; then
  sed -i 's/^maxParallelImagePulls:.*/maxParallelImagePulls: 5/' /var/lib/kubelet/config.yaml
else
  echo 'maxParallelImagePulls: 5' >> /var/lib/kubelet/config.yaml
fi
# unattended-upgrades ships InhibitDelayMaxSec=30 in a logind drop-in that sorts after kubelet's 99-kubelet.conf
mkdir -p /etc/systemd/logind.conf.d
ln -sf /dev/null /etc/systemd/logind.conf.d/unattended-upgrades-logind-maxdelay.conf
systemctl restart kubelet
