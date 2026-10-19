#!/usr/bin/env bash

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

function usage() {
  cat <<EOS
Usage:
   join-worker.sh --control-plane-ip <ip> --gateway <host> --kubernetes-version <version> --containerd-version <version> --runc-version <version> --cgroup-driver <driver> --token-file <path> --discovery-token-ca-cert-hash <hash> --registry-credentials <path>

Join this Debian 12 host to the minikube cluster as a tainted worker; files/usr/local/bin/minikube-worker.sh runs it with the arguments
EOS
}

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
printf 'net.ipv4.ip_forward = 1\nnet.bridge.bridge-nf-call-iptables = 1\n' > /etc/sysctl.d/kubernetes.conf
sysctl --system > /dev/null

apt-get update -q
apt-get install -yq ca-certificates curl gpg conntrack iptables socat ethtool avahi-daemon libnss-mdns

curl -fsSL --retry 5 --retry-all-errors -o /tmp/containerd.tar.gz "https://github.com/containerd/containerd/releases/download/v${containerd_version}/containerd-${containerd_version}-linux-amd64.tar.gz"
tar zxf /tmp/containerd.tar.gz -C /usr/local
curl -fsSL --retry 5 --retry-all-errors -o /etc/systemd/system/containerd.service "https://raw.githubusercontent.com/containerd/containerd/v${containerd_version}/containerd.service"
curl -fsSL --retry 5 --retry-all-errors -o /usr/local/sbin/runc "https://github.com/opencontainers/runc/releases/download/v${runc_version}/runc.amd64"
chmod 755 /usr/local/sbin/runc
mkdir -p /etc/containerd
containerd config default > /etc/containerd/config.toml
sed -i "s/SystemdCgroup = false/SystemdCgroup = ${systemd_cgroup}/" /etc/containerd/config.toml
grep -q "SystemdCgroup = ${systemd_cgroup}" /etc/containerd/config.toml
systemctl daemon-reload
systemctl enable --now containerd

mkdir -p /etc/apt/keyrings
curl -fsSL --retry 5 --retry-all-errors -o /tmp/kubernetes.key "https://pkgs.k8s.io/core:/stable:/${kubernetes_version%.*}/deb/Release.key"
gpg --batch --yes --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg /tmp/kubernetes.key
echo "deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/${kubernetes_version%.*}/deb/ /" > /etc/apt/sources.list.d/kubernetes.list
apt-get update -q
apt-get install -yq --allow-change-held-packages "kubelet=${kubernetes_version#v}-1.1" "kubeadm=${kubernetes_version#v}-1.1"
apt-mark hold kubelet kubeadm
systemctl enable kubelet

cat <<EOS > /usr/local/sbin/minikube-route
#!/usr/bin/env bash

set -Eeo pipefail
trap 'echo "exit \$?: \$BASH_COMMAND(line \$LINENO)" >&2' ERR

resolved=\$(getent ahostsv4 ${gateway})
ip route replace ${control_plane_ip%.*}.0/24 via "\$(awk 'NR == 1 {print \$1}' <<< "\$resolved")"
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

sed -i '/ control-plane\.minikube\.internal$/d' /etc/hosts
echo "${control_plane_ip} control-plane.minikube.internal" >> /etc/hosts

# minikube's kubelet-config sets authentication.x509.clientCAFile to this path and kubeadm join keeps it: https://github.com/kubernetes/kubernetes/blob/v1.37.1/cmd/kubeadm/app/componentconfigs/kubelet.go#L146-L150
mkdir -p /var/lib/minikube/certs
ln -sf /etc/kubernetes/pki/ca.crt /var/lib/minikube/certs/ca.crt

install -D -m 600 "$registry_credentials" /var/lib/kubelet/config.json

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
      effect: NoSchedule
    # https://docs.cilium.io/en/stable/installation/taints/
    - key: node.cilium.io/agent-not-ready
      value: "true"
      effect: NoExecute
EOS
kubeadm join --config "$join_configuration"
