#!/usr/bin/env bash

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

readonly _PACKAGES=(
  # Languages
  protobuf-compiler
  mold
  gcc-mingw-w64-x86-64
  golang-go
  ruby
  python3
  python3-dev
  python3-pip
  gradle
  pandoc
  # Development tools
  man
  git
  netcat-openbsd
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
  rclone
  unzip
  gh
  cloudflared
  dnsutils
  terraform
  ttyd
  # Kubernetes
  socat
  # Capture tools
  xclip
  libjpeg-turbo-progs
  gifsicle
  ffmpeg
  libimage-exiftool-perl
  # Observability tools
  valgrind
  graphviz
  linux-tools-common
  strace
  libbpf-dev
  bpftrace
  dwarves
  microsocks
  # Dependencies
  build-essential
  libffi-dev
  libyaml-dev
  openssh-client
)

# https://launchpad.net/~fish-shell/+archive/ubuntu/release-3
curl -fsSL "https://keyserver.ubuntu.com/pks/lookup?op=get&search=0x88421E703EDC7AF54967DED473C9FCC9E2BB48DA" | gpg --dearmor --yes -o /usr/share/keyrings/fish-archive-keyring.gpg
echo "deb [signed-by=/usr/share/keyrings/fish-archive-keyring.gpg] https://ppa.launchpadcontent.net/fish-shell/release-3/ubuntu/ jammy main" > /etc/apt/sources.list.d/fish.list
# https://docs.cloud.google.com/sdk/docs/install
curl -fsSL https://packages.cloud.google.com/apt/doc/apt-key.gpg | gpg --dearmor --yes -o /usr/share/keyrings/google-cloud-cli.gpg
echo "deb [signed-by=/usr/share/keyrings/google-cloud-cli.gpg] https://packages.cloud.google.com/apt cloud-sdk main" > /etc/apt/sources.list.d/google-cloud-cli.list
# https://github.com/cli/cli/blob/trunk/docs/install_linux.md
curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg | gpg --dearmor --yes -o /usr/share/keyrings/githubcli-archive-keyring.gpg
echo "deb [signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" > /etc/apt/sources.list.d/github-cli.list
# https://pkg.cloudflare.com/index.html
curl -fsSL https://pkg.cloudflare.com/cloudflare-main.gpg | gpg --dearmor --yes -o /usr/share/keyrings/cloudflare-main.gpg
echo "deb [signed-by=/usr/share/keyrings/cloudflare-main.gpg] https://pkg.cloudflare.com/cloudflared jammy main" > /etc/apt/sources.list.d/cloudflared.list
# https://developer.hashicorp.com/terraform/install
curl -fsSL https://apt.releases.hashicorp.com/gpg | gpg --dearmor --yes -o /usr/share/keyrings/hashicorp-archive-keyring.gpg
echo "deb [signed-by=/usr/share/keyrings/hashicorp-archive-keyring.gpg] https://apt.releases.hashicorp.com jammy main" > /etc/apt/sources.list.d/hashicorp.list

# an already-provisioned host still carries the nvidia-container-toolkit source this script no longer declares
rm -f /etc/apt/sources.list.d/nvidia-container-toolkit.list /usr/share/keyrings/nvidia-container-toolkit.gpg

apt-get update -y
apt-get upgrade -y
apt-get install -y --no-install-recommends "${_PACKAGES[@]}"

readonly _WATCHEXEC_VERSION=2.5.1
curl -fsSL "https://github.com/watchexec/watchexec/releases/download/v${_WATCHEXEC_VERSION}/watchexec-${_WATCHEXEC_VERSION}-x86_64-unknown-linux-gnu.tar.xz" | tar Jxf - -C /usr/local/bin --strip-components=1 --wildcards "*/watchexec"

curl -fsSL --retry 5 --retry-all-errors --remove-on-error https://storage.googleapis.com/minikube/releases/latest/minikube-linux-amd64 -o /usr/local/bin/minikube
chmod +x /usr/local/bin/minikube

curl -fsSL --retry 5 --retry-all-errors --remove-on-error https://github.com/asciinema/asciinema/releases/latest/download/asciinema-x86_64-unknown-linux-gnu -o /usr/local/bin/asciinema
chmod +x /usr/local/bin/asciinema
curl -fsSL --retry 5 --retry-all-errors --remove-on-error https://github.com/asciinema/agg/releases/latest/download/agg-x86_64-unknown-linux-gnu -o /usr/local/bin/agg
chmod +x /usr/local/bin/agg
