#!/usr/bin/env bash

set -e

sync -f

if [ -f /boot/loader/entries/arch.conf ]; then
  devicename=$(cat /boot/loader/entries/arch.conf | grep cryptdevice | sed -r 's|.*cryptdevice=/dev/mapper/([^\s]*):.*|\1|')
  vgname=${devicename%-*}
  lvname=${devicename##*-}

  # Disable snapshot
  lvs ${vgname}/pre-install-snapshot-${lvname} > /dev/null 2>&1 || lvcreate -s -L 800G -n pre-install-snapshot-${lvname} /dev/${vgname}/${lvname}
fi

ENTRYPOINT=$(cd $(dirname ${BASH_SOURCE[0]}); pwd)

_SHARED_GROUP=hippocampus
_USERS=(
  kai
  kai-rdp
)
export _USER=${_USERS[0]}

_USER_TARGETS=(
  .ideavimrc
  .gitconfig
  .gitignore
  .xinitrc
  .asdfrc
  .bash_profile
  .tmux.conf
  .tmux.dev.conf
  .tool-versions
  .asdf/plugins/hippocampus
  .asdf/plugins/taurin
  .config/openbox/rc.xml
  .config/openbox/menu.xml
  .config/fish/config.fish
  .config/fish/completions/k9s.fish
  .config/fish/completions/kubectl.fish
  .config/k9s/plugins.yaml
  .config/k9s/skin.yaml
  .config/fcitx5/profile
  .config/fcitx5/config
  .config/fcitx5/conf/classicui.conf
  .config/fcitx5/conf/clipboard.conf
  .config/fcitx5/conf/mozc.conf
  .config/fcitx5/conf/quickphrase.conf
  .config/git/template
  .config/dunst/dunstrc
  .config/easyeffects/db/autogainrc
  .config/easyeffects/db/compressorrc
  .config/easyeffects/db/easyeffectsrc
  .config/easyeffects/db/gaterc
  .config/easyeffects/db/rnnoiserc
  .config/JetBrains/IntelliJIdea2025.3/keymaps/Mine.xml
  .config/JetBrains/IntelliJIdea2025.3/templates
  .config/claudex/agents
  .config/claudex/commands
  .config/claudex/hooks
  .config/claudex/CLAUDE.md
  .config/claudex/CLAUDE.important.md
  .config/claudex/CLAUDE.general.md
  .config/claudex/settings.json
  .minikube/files/usr/local/bin/init.sh
  .minikube/files/usr/local/bin/prune.sh
  .codex/config.toml
  .codex/AGENTS.md
  .gemini/settings.json
  .gemini/AGENTS.md
  bin/claudex
  bin/claudex-control-plane
  bin/claudex-discussion
  bin/claudex-with-worktree
  bin/dux
  bin/hacklaudex
  bin/merge
  bin/safenv
  bin/xkill
  llm
  kustomize/plugin/kustomize.kaidotio.github.io/v1/secretsfromvault/SecretsFromVault
)

_SYSTEM_TARGETS=(
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
  /etc/profile.d/android.sh
  /etc/profile.d/display.sh
  /etc/profile.d/editor.sh
  /etc/profile.d/gemini.sh
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

setup_user_targets() {
  target_user=$1
  home_dir=/home/${target_user}

  for target in "${_USER_TARGETS[@]}"; do
    dest=${home_dir}/${target}
    src=${ENTRYPOINT}/files/home/${_USER}/${target}
    dir=$(dirname ${dest})

    if [ ! -d ${dir} ]; then
      mkdir -p ${dir}
      chown -R ${target_user}:${target_user} ${dir}
    fi

    if [ -L ${dest} ]; then
      rm ${dest}
    fi
    if [ -d ${dest} ]; then
      rmdir ${dest}
    fi
    if [ -f ${dest} ]; then
      rm ${dest}
    fi
    ln -s ${src} ${dest}
  done

  chown -R ${target_user}:${target_user} ${home_dir}
}

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

for user in "${_USERS[@]}"; do
  id ${user} || (useradd ${user} && passwd ${user})
  mkdir -p /home/${user}
  chown -R ${user}:${user} /home/${user}
done

# ---

sed -r 's/# (%wheel ALL=\(ALL:ALL\) NOPASSWD: ALL)/\1/' /etc/sudoers | EDITOR="tee" visudo > /dev/null

# ---

for user in "${_USERS[@]}"; do
  groups ${user} | grep wheel || usermod -aG wheel ${user}
done

# ---

getent group ${_SHARED_GROUP} || groupadd ${_SHARED_GROUP}
for user in "${_USERS[@]}"; do
  groups ${user} | grep ${_SHARED_GROUP} || usermod -aG ${_SHARED_GROUP} ${user}
done

# ---

name=$(head -n1 /etc/os-release)
if [[ "${name}" =~ "Arch Linux" ]]; then
  . ${ENTRYPOINT}/setup/arch/env.sh
  bash ${ENTRYPOINT}/setup/arch/install-packages.sh

  if [ ! -d /var/lib/machines/colab/etc ]; then
    mkdir -p /var/lib/machines/colab
    chown ${_USER}:${_USER} /var/lib/machines/colab
    pacstrap -c /var/lib/machines/colab
  fi
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

for user in "${_USERS[@]}"; do
  setup_user_targets "${user}"
done

for target in "${_SYSTEM_TARGETS[@]}"; do
  dir=$(dirname ${target})
  if [ ! -d ${dir} ]; then
    mkdir -p ${dir}
    chown -R ${_USER}:${_SHARED_GROUP} ${dir}
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

for user in "${_USERS[@]}"; do
  for group in "${_GROUPS[@]}"; do
    groups ${user} | grep ${group} || usermod -aG ${group} ${user}
  done
done

# ---

for user in "${_USERS[@]}"; do
  chown -R ${user}:${user} /home/${user}
done
chown -R ${_USER}:${_USER} ${ENTRYPOINT}
chattr +i ${ENTRYPOINT}/files/home/${_USER}/.config/fcitx5/profile
ln -sf /usr/bin/google-chrome-stable /usr/bin/google-chrome

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
