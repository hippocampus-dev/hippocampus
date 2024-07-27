#!/usr/bin/env bash

set -eo pipefail

sync

devicename=$(cat /boot/loader/entries/arch.conf | grep cryptdevice | sed -r 's|.*cryptdevice=/dev/mapper/([^\s]*):.*|\1|')
vgname="${devicename%-*}"
lvname="${devicename##*-}"

# Disable snapshot
#lvcreate -s -L 800G -n pre-install-snapshot-${lvname} /dev/${vgname}/${lvname}

ENTRYPOINT=$(cd $(dirname ${BASH_SOURCE[0]}); pwd)
_USER=kai
PACKAGES=(
  # Installation
  gdisk
  dosfstools
  arch-install-scripts
  archlinux-keyring
  # Graphics
  nvidia
  # X
  xorg-server
  xorg-xinit
  openbox
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
  rustup
  mold
  protobuf
  go
  python
  python-pip
  nodejs
  npm
  deno
  ruby
  cuda
  cudnn
  # QEMU
  libvirt
  virt-manager
  qemu-base
  qemu-hw-usb-host
  dnsmasq
  dmidecode
  # Development tools
  man
  git
  github-cli
  curl
  jq
  fish
  tmux
  vim
  fzf
  mkcert
  docker
  docker-compose
  docker-buildx
  openssh
  dnsutils
  nfs-utils
  fuse2
  redis
  minio-client
  rsync
  # Kubernetes
  socat
  iptables-nft
  minikube
  kubectl
  k9s
  kustomize
  skaffold
  helm
  # Capture tools
  xclip
  peek
  inkscape
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
  # Documents
  evince
  # Drivers
  piper
  webkit2gtk-4.1
  # Dependencies
  jre-openjdk
)
AURS=(
  https://aur.archlinux.org/downgrade.git
  https://aur.archlinux.org/google-chrome.git
  https://aur.archlinux.org/intellij-idea-ultimate-edition.git
  https://aur.archlinux.org/google-cloud-cli.git
  https://aur.archlinux.org/kind.git
  https://aur.archlinux.org/docker-machine-driver-kvm2.git
  https://aur.archlinux.org/libnvidia-container.git
  https://aur.archlinux.org/nvidia-container-toolkit.git
  https://aur.archlinux.org/ventoy-bin.git
)
_GROUPS=(
  docker
  libvirt
  wireshark
)
TARGETS=(
  /home/kai/.ideavimrc
  /home/kai/.gitconfig
  /home/kai/.gitignore
  /home/kai/.xinitrc
  /home/kai/.bash_profile
  /home/kai/.tmux.conf
  /home/kai/.tmux.dev.conf
  /home/kai/.config/openbox/rc.xml
  /home/kai/.config/openbox/menu.xml
  /home/kai/.config/fish/config.fish
  /home/kai/.config/fish/completions/k9s.fish
  /home/kai/.config/fish/completions/kubectl.fish
  /home/kai/.config/k9s/plugins.yaml
  /home/kai/.config/k9s/skin.yml
  /home/kai/.config/fcitx5/profile
  /home/kai/.config/fcitx5/config
  /home/kai/.config/fcitx5/conf/classicui.conf
  /home/kai/.config/fcitx5/conf/clipboard.conf
  /home/kai/.config/fcitx5/conf/mozc.conf
  /home/kai/.config/fcitx5/conf/quickphrase.conf
  /home/kai/.config/dunst/dunstrc
  /home/kai/.minikube/files/usr/local/bin/init.sh
  /home/kai/llm
  /home/kai/kustomize/plugin/kustomize.kaidotio.github.io/v1/secretsfromvault/SecretsFromVault
  /usr/local/bin/down-volume
  /usr/local/bin/minikube-socat.sh
  /usr/local/bin/minikube-start.sh
  /usr/local/bin/minikube-stop.sh
  /usr/local/bin/screenshot
  /usr/local/bin/show-volume
  /usr/local/bin/startup.sh
  /usr/local/bin/shutdown.sh
  /usr/local/bin/sync.sh
  /usr/local/bin/up-volume
  /usr/share/dbus-1/services/org.freedesktop.Notifications.service
  /etc/hosts
  /etc/systemd/system/sync.timer
  /etc/systemd/system/sync.service
  /etc/systemd/system/lifecycle.service
  /etc/systemd/system/minikube.service
  /etc/systemd/system/minikube-socat.service
  /etc/docker/daemon.json
  /etc/profile.d/editor.sh
  /etc/profile.d/makeflags.sh
  /etc/profile.d/path.sh
  /etc/profile.d/secrets.sh
  /etc/profile.d/sslkeylog.sh
  /etc/X11/xinit/xserverrc
  /etc/sysctl.d/10_perf.conf
  /etc/bluetooth/main.conf
  /etc/libvirt/hooks/qemu
  /etc/exports
)
SERVICES=(
  docker.service
  minikube.service
  minikube-socat.service
  libvirtd.service
  bluetooth.service
  lifecycle.service
  nfs-server.service
  sync.timer
)

# ---

timedatectl set-ntp true

ip link show

read -e -p "Enter Wireless Network Interface: " -r WIRELESS_NIC
read -e -p "Enter SSID: " -r SSID
iwctl station "$WIRELESS_NIC" connect "$SSID"
cat <<EOS > /etc/systemd/network/$WIRELESS_NIC.network
[Match]
Name=$WIRELESS_NIC

[Network]
DHCP=yes
DNS=1.1.1.1

[DHCPv4]
RouteMetric=10
EOS
systemctl enable --now iwd.service
systemctl enable --now systemd-networkd.service
systemctl enable --now systemd-resolved.service
ln -sf /run/systemd/resolve/stub-resolv.conf /etc/resolv.conf

sleep 10

# ---

id $_USER || (useradd $_USER && passwd $_USER)
mkdir -p /home/${_USER}
chown -R ${_USER}:${_USER} /home/${_USER}

# ---

sed -r 's/# (%wheel ALL=\(ALL:ALL\) NOPASSWD: ALL)/\1/' /etc/sudoers | EDITOR="tee" visudo > /dev/null

# ---

groups $_USER | grep wheel || usermod -aG wheel $_USER

# ---

pacman-key --refresh --keyserver hkp://keyserver.ubuntu.com
pacman -Syu --noconfirm
pacman -S iptables-nft # conflict
pacman -S --noconfirm "${PACKAGES[@]}"

# ---

mkdir -p /srv/nfs
chown -R nobody:nobody /srv/nfs
mkdir -p /srv/nfs/.cache
chown -R nobody:nobody /srv/nfs/.cache

# ---

for aur in "${AURS[@]}"; do
  name=$(echo $aur | rev | cut -d/ -f1 | rev)
  dir=/usr/local/src/${name}
  if [ -d $dir ]; then
    cd $dir
    git pull origin master
  else
    git clone $aur $dir
    cd $dir
  fi
  chown -R ${_USER}:${_USER} $dir
  sudo -u $_USER makepkg -si --noconfirm
done

# ---

for target in "${TARGETS[@]}"; do
  dir=$(dirname ${target})
  if [ ! -d $dir ]; then
    mkdir -p ${dir}
    chown -R ${_USER}:${_USER} ${dir}
  fi

  if [ -d $target ]; then
    rmdir ${target}
  else
    rm -f ${target}
  fi
  ln -s ${ENTRYPOINT}/files${target} ${target}
  chown ${_USER}:${_USER} ${target}
done

# ---

for group in "${_GROUPS[@]}"; do
  groups ${_USER} | grep ${group} || usermod -aG ${group} ${_USER}
done

# ---

for service in "${SERVICES[@]}"; do
  systemctl enable --now ${service}
done

# ---

chown -R ${_USER}:${_USER} /home/${_USER}
chown -R ${_USER}:${_USER} ${ENTRYPOINT}
chattr +i ${ENTRYPOINT}/files/home/${_USER}/.config/fcitx5/profile

# ---

mkdir -p /var/certs
chown -R ${_USER}:${_USER} /var/certs

# ---

mkdir -p /var/lib/machines/colab
chown ${_USER}:${_USER} /var/lib/machines/colab
pacstrap -c /var/lib/machines/colab
cat <<EOS | systemd-nspawn -D /var/lib/machines/colab --pipe
pacman -S --noconfirm git python3 python-pip
pip install jupyterlab jupyter_http_over_ws notebook
jupyter serverextension enable --py jupyter_http_over_ws
id ${_USER} || (useradd ${_USER} && passwd -d ${_USER})
mkdir -p /home/${_USER}
chown -R ${_USER}:${_USER} /home/${_USER}
EOS

mkdir -p /etc/systemd/nspawn
cat <<EOS > /etc/systemd/nspawn/colab.nspawn
[Exec]
User=kai
Boot=false
Parameters=jupyter notebook --NotebookApp.allow_origin="https://colab.research.google.com" --port=8888 --NotebookApp.port_retries=0

[Network]
VirtualEthernet=no
EOS
machinectl enable colab

#---

mkdir -p /var/lib/libvirt/images
virsh net-autostart default

#---

bpftool btf dump file /sys/kernel/btf/vmlinux format c > /usr/include/vmlinux.h

#---

sudo -u $_USER bash <<EOS
cd /var/certs
mkcert -install
mkcert 127.0.0.1
mkcert "*.127.0.0.1.nip.io"
mkcert "*.minikube.127.0.0.1.nip.io"
cat /var/certs/127.0.0.1.pem /var/certs/127.0.0.1-key.pem > /var/certs/127.0.0.1-concatenate.pem
cat /var/certs/_wildcard.127.0.0.1.nip.io.pem /var/certs/_wildcard.127.0.0.1.nip.io-key.pem > /var/certs/_wildcard.127.0.0.1.nip.io-concatenate.pem
cat /var/certs/_wildcard.minikube.127.0.0.1.nip.io.pem /var/certs/_wildcard.minikube.127.0.0.1.nip.io-key.pem > /var/certs/_wildcard.minikube.127.0.0.1.nip.io-concatenate.pem

mkdir -p /home/${_USER}/.ssh
chmod 700 /home/${_USER}/.ssh
touch /home/${_USER}/.ssh/config
chmod 600 /home/${_USER}/.ssh/config
cat <<EOF >> /home/${_USER}/.ssh/config
Host github.com
  User git
  IdentityFile /home/${_USER}/.ssh/github
EOF
ssh-keygen -t ed25519 -f /home/${_USER}/.ssh/github -N ""
mkdir -p /home/${_USER}/bin
mkdir -p /home/${_USER}/.secrets
chmod 700 /home/${_USER}/.secrets

rustup default nightly
cargo install wasm-pack
cargo install cross
cargo install cargo-expand
cargo install cargo-watch
# Dependencies
cargo install grcov
cargo install evcxr_repl

pip install --upgrade poetry --break-system-packages

sudo npm install -g @google/clasp
curl -fsSL https://downloads.slack-edge.com/slack-cli/install.sh | bash
EOS

reboot
