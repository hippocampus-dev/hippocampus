#!/usr/bin/env bash

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

_PACKAGES=(
  # Installation
  gdisk
  dosfstools
  arch-install-scripts
  archlinux-keyring
  btrfs-progs
  apparmor
  linux-headers
  # Graphics
  nvidia-open
  libnvidia-container
  nvidia-container-toolkit
  # X
  xorg-server
  xorg-xinit
  xorg-xlsclients
  openbox
  wmctrl
  python-pyxdg
  # Inputs
  noto-fonts
  noto-fonts-cjk
  noto-fonts-emoji
  noto-fonts-extra
  fcitx5
  fcitx5-im
  fcitx5-mozc
  # Notification
  libnotify
  dunst
  pavucontrol
  pipewire
  pipewire-alsa
  pipewire-pulse
  wireplumber
  easyeffects
  #pulseaudio-alsa
  #pulseaudio-bluetooth
  bluez
  bluez-utils
  # Languages
  protobuf
  mold
  go
  python
  python-pip
  cuda
  cudnn
  gradle
  pandoc
  # QEMU
  libvirt
  virt-manager
  qemu-base
  qemu-hw-usb-host
  dnsmasq
  dmidecode
  # Development tools
  xterm
  man
  git
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
  docker
  docker-compose
  docker-buildx
  slirp4netns
  fuse-overlayfs
  nfs-utils
  fuse2
  unzip
  github-cli
  cloudflared
  dnsutils
  watchexec
  terraform
  ttyd
  freerdp
  # Kubernetes
  minikube
  socat
  # Capture tools
  xclip
  imagemagick
  libjpeg-turbo
  gifsicle
  ffmpeg
  v4l-utils
  v4l2loopback-dkms
  inotify-tools
  asciinema
  # Observability tools
  valgrind
  kcachegrind
  graphviz
  perf
  strace
  bpf
  bpftrace
  pahole
  wireshark-qt
  microsocks
  # Driver
  piper
  steam
  celluloid
  musescore
  libbsd
  # Dependencies
  webkit2gtk-4.1
  openssh
)

_AURS=(
  https://aur.archlinux.org/downgrade.git
  https://aur.archlinux.org/google-chrome.git
  https://aur.archlinux.org/android-studio.git
  https://aur.archlinux.org/intellij-idea-ultimate-edition.git
  https://aur.archlinux.org/google-cloud-cli.git
  https://aur.archlinux.org/docker-machine-driver-kvm2.git
  https://aur.archlinux.org/zsa-keymapp-bin.git
  https://aur.archlinux.org/asciinema-agg.git
  https://aur.archlinux.org/xrdp.git
  https://aur.archlinux.org/xorgxrdp.git
  # xRDP audio (PipeWire)
  https://aur.archlinux.org/pipewire-module-xrdp-git.git
  # xRDP audio (PulseAudio) - uncomment to use PulseAudio instead
  #https://aur.archlinux.org/pulseaudio-module-xrdp.git
)

pacman-key --init
pacman-key --populate archlinux
pacman-key --refresh
pacman -Sy --noconfirm archlinux-keyring
pacman -Su --noconfirm
pacman -S --noconfirm --ask 4 iptables-nft netcat # conflict

# IgnorePkg applies to --sysupgrade alone, and --noconfirm answers its "Install anyway?" prompt for a named target with yes
ignored=$(pacman-conf IgnorePkg)
targets=()
for package in "${_PACKAGES[@]}"; do
  pinned=""
  while IFS= read -r pattern; do
    case "$package" in
      $pattern)
        pinned=1
        break
        ;;
    esac
  done <<< "$ignored"
  if [ -n "$pinned" ] && pacman -Qq "$package" > /dev/null 2>&1; then
    continue
  fi
  targets+=("$package")
done
pacman -S --noconfirm "${targets[@]}"

sudo -u "$_USER" gpg --recv-keys 61ECEABBF2BB40E3A35DF30A9F72CDBC01BF10EB # xorgxrdp

for aur in "${_AURS[@]}"; do
  name=$(echo "$aur" | awk -F/ '{print $NF}')
  dir="/usr/local/src/${name}"
  if [ -d "$dir" ]; then
    cd "$dir"
    git pull origin master
  else
    git clone "$aur" "$dir"
    cd "$dir"
  fi
  chown -R "${_USER}:${_USER}" "$dir"
  sudo -u "$_USER" makepkg -siC --noconfirm
done

_DOCKER_ROOTLESS_VERSION=29.7.2
if [ ! -f /usr/local/lib/docker/dockerd-rootless.sh ]; then
  mkdir -p /usr/local/lib/docker
  curl -fsSL "https://download.docker.com/linux/static/stable/$(uname -m)/docker-${_DOCKER_ROOTLESS_VERSION}.tgz" | tar zxf - -C /usr/local/lib/docker --strip-components=1
  curl -fsSL "https://download.docker.com/linux/static/stable/$(uname -m)/docker-rootless-extras-${_DOCKER_ROOTLESS_VERSION}.tgz" | tar zxf - -C /usr/local/lib/docker --strip-components=1
fi
