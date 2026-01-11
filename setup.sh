#!/usr/bin/env bash

set -e

sync -f

if [ -f /boot/loader/entries/arch.conf ]; then
  devicename=$(cat /boot/loader/entries/arch.conf | grep cryptdevice | sed -r 's|.*cryptdevice=/dev/mapper/([^\s]*):.*|\1|')
  vgname=${devicename%-*}
  lvname=${devicename##*-}

  # Disable snapshot
  lvcreate -s -L 800G -n pre-install-snapshot-${lvname} /dev/${vgname}/${lvname}
fi

ENTRYPOINT=$(cd $(dirname ${BASH_SOURCE[0]}); pwd)

export _USER=kai

_TARGETS=(
  /home/${_USER}/.ideavimrc
  /home/${_USER}/.gitconfig
  /home/${_USER}/.gitignore
  /home/${_USER}/.xinitrc
  /home/${_USER}/.asdfrc
  /home/${_USER}/.bash_profile
  /home/${_USER}/.tmux.conf
  /home/${_USER}/.tmux.dev.conf
  /home/${_USER}/.tool-versions
  /home/${_USER}/.asdf/plugins/hippocampus
  /home/${_USER}/.config/openbox/rc.xml
  /home/${_USER}/.config/openbox/menu.xml
  /home/${_USER}/.config/fish/config.fish
  /home/${_USER}/.config/fish/completions/k9s.fish
  /home/${_USER}/.config/fish/completions/kubectl.fish
  /home/${_USER}/.config/k9s/plugins.yaml
  /home/${_USER}/.config/k9s/skin.yaml
  /home/${_USER}/.config/fcitx5/profile
  /home/${_USER}/.config/fcitx5/config
  /home/${_USER}/.config/fcitx5/conf/classicui.conf
  /home/${_USER}/.config/fcitx5/conf/clipboard.conf
  /home/${_USER}/.config/fcitx5/conf/mozc.conf
  /home/${_USER}/.config/fcitx5/conf/quickphrase.conf
  /home/${_USER}/.config/git/template
  /home/${_USER}/.config/dunst/dunstrc
  /home/${_USER}/.config/easyeffects/db/autogainrc
  /home/${_USER}/.config/easyeffects/db/compressorrc
  /home/${_USER}/.config/easyeffects/db/easyeffectsrc
  /home/${_USER}/.config/JetBrains/IntelliJIdea2025.3/keymaps/Mine.xml
  /home/${_USER}/.config/JetBrains/IntelliJIdea2025.3/templates
  /home/${_USER}/.config/claudex/agents
  /home/${_USER}/.config/claudex/commands
  /home/${_USER}/.config/claudex/hooks
  /home/${_USER}/.config/claudex/CLAUDE.md
  /home/${_USER}/.config/claudex/CLAUDE.important.md
  /home/${_USER}/.config/claudex/CLAUDE.general.md
  /home/${_USER}/.config/claudex/settings.json
  /home/${_USER}/.minikube/files/usr/local/bin/init.sh
  /home/${_USER}/.minikube/files/usr/local/bin/prune.sh
  /home/${_USER}/.codex/config.toml
  /home/${_USER}/.gemini/settings.json
  /home/${_USER}/bin/claudex
  /home/${_USER}/bin/claudex-control-plane
  /home/${_USER}/bin/claudex-discussion
  /home/${_USER}/bin/claudex-with-worktree
  /home/${_USER}/bin/dux
  /home/${_USER}/bin/hacklaude
  /home/${_USER}/bin/merge
  /home/${_USER}/bin/safenv
  /home/${_USER}/bin/xkill
  /home/${_USER}/llm
  /home/${_USER}/kustomize/plugin/kustomize.kaidotio.github.io/v1/secretsfromvault/SecretsFromVault
  /usr/local/bin/down-volume
  /usr/local/bin/minikube-context.sh
  /usr/local/bin/minikube-reset.sh
  /usr/local/bin/minikube-socat.sh
  /usr/local/bin/minikube-start.sh
  /usr/local/bin/minikube-stop.sh
  /usr/local/bin/pactl-subscribe.sh
  /usr/local/bin/screenshot
  /usr/local/bin/show-volume
  /usr/local/bin/startup.sh
  /usr/local/bin/shutdown.sh
  /usr/local/bin/backup.sh
  /usr/local/bin/sync.sh
  /usr/local/bin/toggle-easyeffects
  /usr/local/bin/up-volume
  /usr/share/dbus-1/services/org.freedesktop.Notifications.service
  /etc/hosts
  /etc/systemd/system/backup.timer
  /etc/systemd/system/backup.service
  /etc/systemd/system/sync.timer
  /etc/systemd/system/sync.service
  /etc/systemd/system/hippocampus-compose.service
  /etc/systemd/system/lifecycle.service
  /etc/systemd/system/minikube.service
  /etc/systemd/system/minikube-context.service
  /etc/systemd/system/minikube-reset.service
  /etc/systemd/system/minikube-socat.service
  /etc/systemd/user/pactl-subscribe.service
  /etc/systemd/resolved.conf.d/security.conf
  /etc/systemd/resolved.conf.d/tls.conf
  /etc/apparmor.d/claude
  /etc/docker/daemon.json
  /etc/xrdp/sesman.ini
  /etc/profile.d/display.sh
  /etc/profile.d/editor.sh
  /etc/profile.d/makeflags.sh
  /etc/profile.d/path.sh
  /etc/profile.d/secrets.sh
  /etc/profile.d/sslkeylog.sh
  /etc/profile.d/tls.sh
  /etc/profile.d/x.sh
  /etc/X11/xinit/xserverrc
  /etc/sysctl.d/0_port.conf
  /etc/sysctl.d/10_perf.conf
  /etc/bluetooth/main.conf
  /etc/libvirt/hooks/qemu
  /etc/udev/rules.d/50-zsa.rules
  /etc/exports
)

# ---

timedatectl set-ntp true

ip link show

read -e -p "Enter Wireless Network Interface: " -r WIRELESS_NIC
read -e -p "Enter SSID: " -r SSID
iwctl station ${WIRELESS_NIC} connect ${SSID}
cat <<EOS > /etc/systemd/network/${WIRELESS_NIC}.network
[Match]
Name=${WIRELESS_NIC}

[Network]
DHCP=yes
DNS=2606:4700:4700::1111 2001:4860:4860::8888 1.1.1.1 8.8.8.8

[DHCPv4]
RouteMetric=10
EOS
systemctl enable --now iwd.service
systemctl enable --now systemd-networkd.service
systemctl enable --now systemd-resolved.service
ln -sf /run/systemd/resolve/stub-resolv.conf /etc/resolv.conf

sleep 10

# ---

id ${_USER} || (useradd ${_USER} && passwd ${_USER})
mkdir -p /home/${_USER}
chown -R ${_USER}:${_USER} /home/${_USER}

# ---

sed -r 's/# (%wheel ALL=\(ALL:ALL\) NOPASSWD: ALL)/\1/' /etc/sudoers | EDITOR="tee" visudo > /dev/null

# ---

groups ${_USER} | grep wheel || usermod -aG wheel ${_USER}

# ---

name=$(head -n1 /etc/os-release)
if [[ "${name}" =~ "Arch Linux" ]]; then
  . ${ENTRYPOINT}/setup/arch/env.sh
  bash ${ENTRYPOINT}/setup/arch/install-packages.sh

  mkdir -p /var/lib/machines/colab
  chown ${_USER}:${_USER} /var/lib/machines/colab
  pacstrap -c /var/lib/machines/colab
  cat <<EOS | systemd-nspawn -D /var/lib/machines/colab --pipe
pacman -S --noconfirm git python3 python-pip
pip install --break-system-packages jupyterlab jupyter_http_over_ws notebook
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

  mkdir -p /home/${_USER}/.config/openbox/autostart
  ln -sf /usr/lib/openbox/openbox-xdg-autostart /home/${_USER}/.config/openbox/autostart/openbox-xdg-autostart
fi
if [[ "${name}" =~ "Ubuntu" ]]; then
  . ${ENTRYPOINT}/setup/ubuntu/env.sh
  bash ${ENTRYPOINT}/setup/ubuntu/install-packages.sh
fi

# ---

if id nobody; then
  mkdir -p /srv/nfs
  chown -R nobody:nobody /srv/nfs
  mkdir -p /srv/nfs/.cache
  chown -R nobody:nobody /srv/nfs/.cache
fi

# ---

for target in "${_TARGETS[@]}"; do
  dir=$(dirname ${target})
  if [ ! -d ${dir} ]; then
    mkdir -p ${dir}
    chown -R ${_USER}:${_USER} ${dir}
  fi

  if [ -L ${target} ]; then
    rm ${target}
  fi
  if [ -d ${target} ]; then
    rmdir ${target}
  fi
  if [ -f ${target} ]; then
    rm ${target}
  fi
  ln -s ${ENTRYPOINT}/files${target} ${target}
done

# ---

for group in "${_GROUPS[@]}"; do
  groups ${_USER} | grep ${group} || usermod -aG ${group} ${_USER}
done

# ---

chown -R ${_USER}:${_USER} /home/${_USER}
chown -R ${_USER}:${_USER} ${ENTRYPOINT}
chattr +i ${ENTRYPOINT}/files/home/${_USER}/.config/fcitx5/profile
ln -s /usr/bin/google-chrome-stable /usr/bin/google-chrome

# ---

for service in "${_SERVICES[@]}"; do
  systemctl enable --now ${service}
done

# ---

mkdir -p /var/lib/libvirt/images
virsh net-autostart default

# ---

bpftool btf dump file /sys/kernel/btf/vmlinux format c > /usr/include/vmlinux.h

# ---

mkdir -p /var/certs
chown -R ${_USER}:${_USER} /var/certs
mkdir -p /var/google-chrome
chown -R ${_USER}:${_USER} /var/google-chrome

#---

for service in "${_USER_SERVICES[@]}"; do
  sudo -u ${_USER} systemctl enable --user ${service}
done
sudo -u ${_USER} bash -l ${ENTRYPOINT}/setup/asdf.sh
sudo -u ${_USER} bash -l ${ENTRYPOINT}/setup/user.sh

reboot
