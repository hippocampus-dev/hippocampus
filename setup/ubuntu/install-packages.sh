#!/usr/bin/env bash

set -e

_PACKAGES=(
  # Languages
  protobuf-compiler
  mold
  golang-go
  ruby
  python3
  python3-dev
  python3-pip
  # Development tools
  xterm
  man
  git
  netcat
  curl
  wget
  rsync
  make
  cmake
  bc
  jq
  fish
  tmux
  vim
  fzf
  mkcert
  google-cloud-cli
  unzip
  cloudflared
  # Capture tools
  xclip
  # QEMU
  virt-manager
  # Observability tools
  linux-tools-common
  libbpf-dev
  bpftrace
  # Drivers
  nvidia-container-toolkit
  # Dependencies
  build-essential
  libffi-dev
  libyaml-dev
)

# https://launchpad.net/~fish-shell/+archive/ubuntu/release-3
curl -fsSL "https://keyserver.ubuntu.com/pks/lookup?op=get&search=0x88421E703EDC7AF54967DED473C9FCC9E2BB48DA" | gpg --dearmor -o /usr/share/keyrings/fish-archive-keyring.gpg
echo 'deb [signed-by=/usr/share/keyrings/fish-archive-keyring.gpg] https://ppa.launchpadcontent.net/fish-shell/release-3/ubuntu/ jammy main' > /etc/apt/sources.list.d/fish.list
curl -fsSL https://nvidia.github.io/libnvidia-container/gpgkey | gpg --dearmor -o /usr/share/keyrings/nvidia-container-toolkit.gpg
echo 'deb [signed-by=/usr/share/keyrings/nvidia-container-toolkit.gpg] https://nvidia.github.io/libnvidia-container/stable/ubuntu18.04/$(ARCH) /' > /etc/apt/sources.list.d/nvidia-container-toolkit.list
curl -fsSL https://packages.cloud.google.com/apt/doc/apt-key.gpg | gpg --dearmor -o /usr/share/keyrings/google-cloud-cli.gpg
echo 'deb [signed-by=/usr/share/keyrings/google-cloud-cli.gpg] https://packages.cloud.google.com/apt cloud-sdk main' > /etc/apt/sources.list.d/google-cloud-cli.list
curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg | gpg --dearmor -o /usr/share/keyrings/githubcli-archive-keyring.gpg
echo 'deb [signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main' > /etc/apt/sources.list.d/github-cli.list
curl -fsSL https://pkg.cloudflare.com/cloudflare-main.gpg | gpg --dearmor -o /usr/share/keyrings/cloudflare-main.gpg
echo 'deb [signed-by=/usr/share/keyrings/cloudflare-main.gpg] https://pkg.cloudflare.com/cloudflared jammy main' > /etc/apt/sources.list.d/cloudflared.list

apt-get update -y
apt-get upgrade -y
apt-get install -y --no-install-recommends "${_PACKAGES[@]}"
