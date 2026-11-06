#!/usr/bin/env bash

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

readonly GITHUB_USER=kaidotio

usage() {
  cat <<EOS
Usage:
   wsl.sh --nvidia|--radeon

--nvidia installs NVIDIA Container Toolkit and hippocampus-compose.service, which runs stable-diffusion-webui-forge
--radeon installs neither, and removes hippocampus-compose.service left by an earlier --nvidia run
EOS
}

while (( $# )); do
  case "$1" in
    -h|--help)
      usage
      exit 0
      ;;
    --nvidia)
      gpu=nvidia
      shift
      ;;
    --radeon)
      gpu=radeon
      shift
      ;;
    *)
      echo "Unsupported flag $1" 1>&2
      exit 1
      ;;
  esac
done

if [ -z "$gpu" ]; then
  usage 1>&2
  exit 1
fi

apt-get update -y
apt-get install -y --no-install-recommends ca-certificates curl gnupg
install -m 0755 -d /etc/apt/keyrings
# https://docs.docker.com/engine/install/ubuntu/
curl -fsSL --retry 5 --retry-all-errors https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
chmod a+r /etc/apt/keyrings/docker.asc
cat <<EOS > /etc/apt/sources.list.d/docker.sources
Types: deb
URIs: https://download.docker.com/linux/ubuntu
Suites: $(. /etc/os-release && echo "${UBUNTU_CODENAME:-$VERSION_CODENAME}")
Components: stable
Architectures: $(dpkg --print-architecture)
Signed-By: /etc/apt/keyrings/docker.asc
EOS
apt-get update -y
# Mirrored networking shares ports with Windows, whose sshd holds 22, and openssh-server's postinst restarts ssh.socket on the port sshd-socket-generator reads from here
install -m 0755 -d /etc/ssh/sshd_config.d
echo "Port 2222" > /etc/ssh/sshd_config.d/port.conf
apt-get install -y --no-install-recommends docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin openssh-server
install -m 0700 -d /root/.ssh
curl -fsSL --retry 5 --retry-all-errors "https://github.com/${GITHUB_USER}.keys" -o /root/.ssh/authorized_keys
if [ "$gpu" = "nvidia" ]; then
  # https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/install-guide.html
  curl -fsSL https://nvidia.github.io/libnvidia-container/gpgkey | gpg --dearmor --yes -o /usr/share/keyrings/nvidia-container-toolkit-keyring.gpg
  curl -fsSL --retry 5 --retry-all-errors https://nvidia.github.io/libnvidia-container/stable/deb/nvidia-container-toolkit.list | sed 's#deb https://#deb [signed-by=/usr/share/keyrings/nvidia-container-toolkit-keyring.gpg] https://#g' > /etc/apt/sources.list.d/nvidia-container-toolkit.list
  apt-get update -y
  apt-get install -y --no-install-recommends nvidia-container-toolkit
  nvidia-ctk runtime configure --runtime=docker
  systemctl restart docker
  cat <<EOS > /etc/systemd/system/hippocampus-compose.service
[Unit]
Requires=docker.service
After=docker.service

[Service]
Restart=always
RestartSec=60s

WorkingDirectory=/opt/hippocampus
ExecStartPre=/usr/bin/docker container prune -f
ExecStart=/usr/bin/docker compose -f docker-compose.yaml -f docker-compose/docker-compose.stable-diffusion-webui-forge.hyperv-host.yaml --profile stable-diffusion-webui-forge up --build stable-diffusion-webui-forge stable-diffusion-webui-forge-downloader
ExecStopPost=/usr/bin/docker compose -f docker-compose.yaml -f docker-compose/docker-compose.stable-diffusion-webui-forge.hyperv-host.yaml down

[Install]
WantedBy=multi-user.target
EOS
  systemctl daemon-reload
  systemctl enable hippocampus-compose.service
elif [ -f /etc/systemd/system/hippocampus-compose.service ]; then
  systemctl disable --now hippocampus-compose.service
  rm /etc/systemd/system/hippocampus-compose.service
  systemctl daemon-reload
fi
