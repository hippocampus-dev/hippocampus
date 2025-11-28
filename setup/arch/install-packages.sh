#!/usr/bin/env bash

set -e

_PACKAGES=(
  # Installation
  gdisk
  dosfstools
  arch-install-scripts
  archlinux-keyring
  btrfs-progs
  apparmor
  # Graphics
  nvidia
  libnvidia-container
  nvidia-container-toolkit
  # X
  xorg-server
  xorg-xinit
  xorg-xlsclients
  openbox
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
  # Audio
  pavucontrol
  pulseaudio-alsa
  pulseaudio-bluetooth
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
  docker
  docker-compose
  docker-buildx
  nfs-utils
  fuse2
  unzip
  github-cli
  cloudflared
  dnsutils
  watchexec
  terraform
  # Kubernetes
  minikube
  socat
  iptables-nft
  # Capture tools
  xclip
  imagemagick
  # Observability tools
  valgrind
  kcachegrind
  graphviz
  perf
  strace
  bpf
  bpftrace
  wireshark-qt
  # Driver
  piper
  steam
  libbsd
  # Dependencies
  webkit2gtk-4.1
  jre-openjdk
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
  https://aur.archlinux.org/xrdp.git
  https://aur.archlinux.org/xorgxrdp.git
  https://aur.archlinux.org/pulseaudio-module-xrdp.git
)

pacman-key --refresh
pacman -Syu --noconfirm
pacman -S iptables-nft # conflict
pacman -S --noconfirm "${_PACKAGES[@]}"

gpg --recv-keys 03993B4065E7193B # xorgxrdp

for aur in "${_AURS[@]}"; do
  name=$(echo ${aur} | awk -F/ '{print $NF}')
  dir=/usr/local/src/${name}
  if [ -d ${dir} ]; then
    cd ${dir}
    git pull origin master
  else
    git clone ${aur} ${dir}
    cd ${dir}
  fi
  chown -R ${_USER}:${_USER} ${dir}
  sudo -u ${_USER} makepkg -si --noconfirm
done
