#!/usr/bin/env bash

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

sync -f

if [ -f /boot/loader/entries/arch.conf ]; then
  device_name=$(cat /boot/loader/entries/arch.conf | awk '/cryptdevice/' | sed -r 's|.*cryptdevice=/dev/mapper/([^:]+):.*|\1|')
  volume_group_name="${device_name%-*}"
  logical_volume_name="${device_name##*-}"

  # Disable snapshot
  #lvs ${volume_group_name}/pre-install-snapshot-${logical_volume_name} > /dev/null 2>&1 || lvcreate -s -L 800G -n pre-install-snapshot-${logical_volume_name} /dev/${volume_group_name}/${logical_volume_name}
fi

ENTRYPOINT=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
readonly ENTRYPOINT

readonly _SHARED_GROUP=hippocampus
readonly _USERS=(
  kai
  kai-rdp
)
export _USER="${_USERS[0]}"

readonly _USER_TARGETS=(
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
  .config/picom/picom.conf
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
  .local/share/fcitx5/data/quickphrase.d/custom.mb
  .config/git/template
  .config/dunst/dunstrc
  .config/fontconfig/fonts.conf
  .config/freerdp/default.rdp
  .config/autostart/chatgpt.desktop
  .config/autostart/com.github.wwmm.easyeffects.desktop
  .config/easyeffects/db/autogainrc
  .config/easyeffects/db/compressorrc
  .config/easyeffects/db/easyeffectsrc
  .config/easyeffects/db/gaterc
  .config/easyeffects/db/limiterrc
  .config/easyeffects/db/rnnoiserc
  .config/JetBrains/IntelliJIdea2025.3/keymaps/Mine.xml
  .config/JetBrains/IntelliJIdea2025.3/templates
  .config/claudex/config/agents
  .config/claudex/config/commands
  .config/claudex/config/skills
  .config/claudex/config/hooks
  .config/claudex/config/CLAUDE.md
  .config/claudex/config/CLAUDE.important.md
  .config/claudex/config/CLAUDE.summary.md
  .config/claudex/config/CLAUDE.general.md
  .config/claudex/config/settings.json
  .minikube/files/usr/local/bin/init.sh
  .minikube/files/usr/local/bin/kubeadm.sh
  .minikube/files/usr/local/bin/prune.sh
  .codex/agents
  .codex/prompts
  .codex/config.toml
  .codex/claudex.config.toml
  .codex/hooks.json
  .codex/keybindings.json
  .codex/AGENTS.md
  .gemini/agents
  .gemini/commands
  .gemini/settings.json
  .gemini/AGENTS.md
  .gemini/config/agents
  .gemini/config/commands
  .gemini/config/hooks.json
  .gemini/GEMINI.md
  .gemini/antigravity-cli/settings.json
  .config/opencode/agents
  .config/opencode/commands
  .config/opencode/skills
  .config/opencode/opencode.json
  .config/opencode/service.json
  .config/opencode/AGENTS.md
  .config/opencode/plugins/claudex-hooks.js
  bin/claudex
  bin/claudex-control-plane
  bin/claudex-discussion
  bin/claudex-with-worktree
  bin/dux
  bin/hacklaudex
  bin/merge
  bin/safenv
  bin/tmux-idea
  bin/tmux-xdg-open
  bin/xdg-open
  bin/xkill
  llm
  kustomize/plugin/kustomize.kaidotio.github.io/v1/secretsfromvault/SecretsFromVault
)

readonly _SYSTEM_TARGETS=(
  /usr/local/bin/down-volume
  /usr/local/bin/spotify-down-volume
  /usr/local/bin/spotify-now-playing
  /usr/local/bin/spotify-play
  /usr/local/bin/spotify-toggle
  /usr/local/bin/spotify-up-volume
  /usr/local/bin/hyperv-host.sh
  /usr/local/bin/minikube-add-hyperv-worker.sh
  /usr/local/bin/minikube-context.sh
  /usr/local/bin/minikube-gce-worker.sh
  /usr/local/bin/minikube-orphan.sh
  /usr/local/bin/minikube-reset.sh
  /usr/local/bin/minikube-socat.sh
  /usr/local/bin/minikube-start.sh
  /usr/local/bin/minikube-stop.sh
  /usr/local/bin/minikube-wake-worker.sh
  /usr/local/bin/minikube-worker.sh
  /usr/local/bin/mirror-registry-garbage-collect.sh
  /usr/local/bin/open-notification-url
  /usr/local/bin/pactl-subscribe.sh
  /usr/local/bin/screenrecord
  /usr/local/bin/screenshot
  /usr/local/bin/setup-hyperv-host.sh
  /usr/local/bin/setup-hyperv-host-wsl.sh
  /usr/local/bin/setup-steam-frame.sh
  /usr/local/bin/setup-windows.sh
  /usr/local/bin/square-webcam.sh
  /usr/local/bin/startup.sh
  /usr/local/bin/shutdown.sh
  /usr/local/bin/backup.sh
  /usr/local/bin/cpu-frequency-limit.sh
  /usr/local/bin/claude-daily-task.sh
  /usr/local/bin/poweroff-after-claudex.sh
  /usr/local/bin/resume-pending-claudex.sh
  /usr/local/bin/sync.sh
  /usr/local/bin/thin-pool-notify.sh
  /usr/local/bin/toggle-easyeffects
  /usr/local/bin/toggle-mute
  /usr/local/bin/up-volume
  /usr/local/bin/wifi-qr.sh
  /usr/local/lib/hyperv-host/lib.sh
  /usr/local/lib/minikube-worker/lib.sh
  /usr/share/dbus-1/services/org.freedesktop.Notifications.service
  /etc/hosts
  /etc/nftables.conf
  /etc/systemd/system/backup.timer
  /etc/systemd/system/backup.service
  /etc/systemd/system/mirror-registry-garbage-collect.timer
  /etc/systemd/system/mirror-registry-garbage-collect.service
  /etc/systemd/system/scrub.timer
  /etc/systemd/system/scrub.service
  /etc/systemd/system/sync.timer
  /etc/systemd/system/sync.service
  /etc/systemd/system/cpu-frequency-limit.service
  /etc/systemd/system/thin-pool-notify.service
  /etc/systemd/system/docker-rootless.service
  /etc/systemd/system/hippocampus-compose.service
  /etc/systemd/system/lifecycle.service
  /etc/systemd/system/minikube.service
  /etc/systemd/system/minikube-context.service
  /etc/systemd/system/minikube-orphan.service
  /etc/systemd/system/minikube-reset.service
  /etc/systemd/system/minikube-socat.service
  /etc/systemd/system/minikube-worker.timer
  /etc/systemd/system/minikube-worker.service
  /etc/systemd/system/minikube-gce-worker.timer
  /etc/systemd/system/minikube-gce-worker.service
  /etc/systemd/system/notify-failure@.service
  /etc/systemd/system/service.d/notify-failure.conf
  /etc/systemd/user/claude-daily-task.timer
  /etc/systemd/user/claude-daily-task.service
  /etc/systemd/system/microsocks.service
  /etc/systemd/system/run-claudex-pts.mount
  /etc/systemd/user/pactl-subscribe.service
  /etc/systemd/user/square-webcam.service
  /etc/systemd/user/rclone-gdrive.service
  /etc/systemd/user/ttyd.service
  /etc/systemd/user/teleput-server.service
  /etc/systemd/user/confidant.service
  /etc/systemd/user/x11vnc.service
  /etc/systemd/user/notify-failure@.service
  /etc/systemd/user/service.d/notify-failure.conf
  /etc/systemd/user/dbus-@.service.d/notify-failure.conf
  /etc/systemd/resolved.conf.d/cache.conf
  /etc/systemd/resolved.conf.d/docker.conf
  /etc/systemd/resolved.conf.d/security.conf
  /etc/systemd/resolved.conf.d/tls.conf
  /etc/systemd/network/eno1.network
  /etc/systemd/network/u3223qz.network
  /etc/tmpfiles.d/tmp.conf
  /etc/tmpfiles.d/armyknife-tmux.conf
  /etc/tmpfiles.d/claudex.conf
  /etc/tmpfiles.d/rclone-gdrive.conf
  /etc/tmpfiles.d/ttyd.conf
  /etc/apparmor.d/claude
  /etc/docker/daemon.json
  /etc/opt/chrome/policies/managed/webrtc.json
  /etc/xrdp/sesman.ini
  /etc/profile.d/android.sh
  /etc/profile.d/display.sh
  /etc/profile.d/editor.sh
  /etc/profile.d/gemini.sh
  /etc/profile.d/makeflags.sh
  /etc/profile.d/ollama.sh
  /etc/profile.d/path.sh
  /etc/profile.d/secrets.sh
  /etc/profile.d/sslkeylog.sh
  /etc/profile.d/tls.sh
  /etc/profile.d/x.sh
  /etc/X11/xinit/xserverrc
  /etc/X11/japannext.edid
  /etc/X11/xorg.conf.d/20-japannext-edid.conf
  /etc/sysctl.d/0_port.conf
  /etc/sysctl.d/10_perf.conf
  /etc/sysctl.d/20_conntrack.conf
  /etc/bluetooth/main.conf
  /etc/iwd/main.conf
  /etc/libvirt/hooks/qemu
  /etc/modprobe.d/v4l2loopback.conf
  /etc/modules-load.d/nct6775.conf
  /etc/modules-load.d/nf_conntrack.conf
  /etc/modules-load.d/v4l2loopback.conf
  /etc/udev/hwdb.d/70-elecom-relacon.hwdb
  /etc/udev/rules.d/50-zsa.rules
  /etc/udev/rules.d/72-webcam-infrared.rules
  /etc/exports
)

setup_user_targets() {
  target_user="$1"
  home_directory="/home/${target_user}"

  for target in "${_USER_TARGETS[@]}"; do
    destination="${home_directory}/${target}"
    source="${ENTRYPOINT}/files/home/${_USER}/${target}"
    directory=$(dirname "$destination")

    if [ ! -d "$directory" ]; then
      mkdir -p "$directory"
      chown -R "${target_user}:${target_user}" "$directory"
    fi

    if [ -L "$destination" ]; then
      rm "$destination"
    fi
    if [ -d "$destination" ]; then
      rmdir "$destination"
    fi
    if [ -f "$destination" ]; then
      rm "$destination"
    fi
    ln -s "$source" "$destination"
  done

  chown -R "${target_user}:${target_user}" "$home_directory"
}

# ---

timedatectl set-ntp true

if [ -z "$(find -L /etc/systemd/network -name "*.network" 2>/dev/null)" ]; then
  ip link show

  read -e -p "Enter Wireless Network Interface: " -r wireless_interface
  read -e -p "Enter SSID: " -r ssid
  iwctl station "$wireless_interface" connect "$ssid"
  cat <<EOS > "/etc/systemd/network/${wireless_interface}.network"
[Match]
Name=${wireless_interface}

[Network]
DHCP=yes
DNS=2606:4700:4700::1111 2001:4860:4860::8888 1.1.1.1 8.8.8.8
MulticastDNS=yes

[DHCPv4]
RouteMetric=10
EOS

  sleep 10
fi

systemctl enable --now iwd.service
systemctl enable --now systemd-networkd.service
systemctl enable --now systemd-resolved.service
ln -sf /run/systemd/resolve/stub-resolv.conf /etc/resolv.conf

# ---

for user in "${_USERS[@]}"; do
  id "$user" || (useradd "$user" && passwd "$user")
  mkdir -p "/home/${user}"
  chown -R "${user}:${user}" "/home/${user}"
done

# ---

sed -r 's/# (%wheel ALL=\(ALL:ALL\) NOPASSWD: ALL)/\1/' /etc/sudoers | EDITOR="tee" visudo > /dev/null

# ---

for user in "${_USERS[@]}"; do
  groups "$user" | grep wheel || usermod -aG wheel "$user"
done

# ---

getent group "$_SHARED_GROUP" || groupadd "$_SHARED_GROUP"
for user in "${_USERS[@]}"; do
  groups "$user" | grep "$_SHARED_GROUP" || usermod -aG "$_SHARED_GROUP" "$user"
done

# ---

name=$(head -n1 /etc/os-release)
if [[ "$name" =~ "Arch Linux" ]]; then
  . "${ENTRYPOINT}/setup/arch/env.sh"
  bash "${ENTRYPOINT}/setup/arch/install-packages.sh"

  if [ ! -d /var/lib/machines/colab/etc ]; then
    mkdir -p /var/lib/machines/colab
    chown "${_USER}:${_USER}" /var/lib/machines/colab
    pacstrap -c /var/lib/machines/colab
    cat <<EOS | systemd-nspawn -D /var/lib/machines/colab --pipe
pacman -S --noconfirm git python3 python-pip
pip install --break-system-packages jupyterlab notebook
id "$_USER" || (useradd "$_USER" && passwd -d "$_USER")
mkdir -p "/home/${_USER}"
chown -R "${_USER}:${_USER}" "/home/${_USER}"
EOS
  fi

  mkdir -p /etc/systemd/nspawn
  cat <<EOS > /etc/systemd/nspawn/colab.nspawn
[Exec]
User=kai
Boot=false
Parameters=jupyter notebook --ServerApp.allow_origin="https://colab.research.google.com" --port=8888 --ServerApp.port_retries=0

[Network]
VirtualEthernet=no
EOS
  machinectl enable colab

  mkdir -p "/home/${_USER}/.config/openbox/autostart"
  ln -sf /usr/lib/openbox/openbox-xdg-autostart "/home/${_USER}/.config/openbox/autostart/openbox-xdg-autostart"
fi
if [[ "$name" =~ "Ubuntu" ]]; then
  . "${ENTRYPOINT}/setup/ubuntu/env.sh"
  bash "${ENTRYPOINT}/setup/ubuntu/install-packages.sh"
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
  setup_user_targets "$user"
done

for target in "${_SYSTEM_TARGETS[@]}"; do
  directory=$(dirname "$target")
  if [ ! -d "$directory" ]; then
    mkdir -p "$directory"
    chown -R "${_USER}:${_SHARED_GROUP}" "$directory"
  fi

  if [ -L "$target" ]; then
    rm "$target"
  fi
  if [ -d "$target" ]; then
    rmdir "$target"
  fi
  if [ -f "$target" ]; then
    rm "$target"
  fi
  ln -s "${ENTRYPOINT}/files${target}" "$target"
done

systemd-hwdb update

# ---

for user in "${_USERS[@]}"; do
  for group in "${_GROUPS[@]}"; do
    groups "$user" | grep "$group" || usermod -aG "$group" "$user"
  done
done

# ---

for user in "${_USERS[@]}"; do
  chown -R "${user}:${user}" "/home/${user}"
done
if [ -f "${ENTRYPOINT}/files/home/${_USER}/.config/fcitx5/profile" ]; then
  chattr -i "${ENTRYPOINT}/files/home/${_USER}/.config/fcitx5/profile"
fi
chown -R "${_USER}:${_USER}" "$ENTRYPOINT"
chattr +i "${ENTRYPOINT}/files/home/${_USER}/.config/fcitx5/profile"
ln -sf /usr/bin/google-chrome-stable /usr/bin/google-chrome

# ---

# docker-rootless.service names this in ReadWritePaths, which fails namespace setup when it is absent
mkdir -p "/home/${_USER}/.local/share/docker"
chown -R "${_USER}:${_USER}" "/home/${_USER}/.local/share"

# ---

for service in "${_SERVICES[@]}"; do
  systemctl enable --now "$service"
done

# ---

mkdir -p /var/lib/libvirt/images
virsh net-autostart default

# The minikube VM resolves through the default network's dnsmasq
# dnsmasq forwards plain names without dots upstream until domain-needed says otherwise
default_network=$(virsh net-dumpxml default)
if ! grep -q 'dnsmasq:options' <<< "$default_network"; then
  printf '%s\n' "$default_network" \
    | sed -e 's|<network\([^>]*\)>|<network\1 xmlns:dnsmasq="http://libvirt.org/schemas/network/dnsmasq/1.0">|' \
          -e 's|</network>|  <dnsmasq:options>\n    <dnsmasq:option value="domain-needed"/>\n  </dnsmasq:options>\n</network>|' \
    > /var/lib/libvirt/network/default.xml
  virsh net-define /var/lib/libvirt/network/default.xml
fi

# ---

bpftool btf dump file /sys/kernel/btf/vmlinux format c > /usr/include/vmlinux.h

# ---

mkdir -p /var/certs
chown -R "${_USER}:${_USER}" /var/certs
mkdir -p /var/google-chrome
chown -R "${_USER}:${_USER}" /var/google-chrome

# ---

loginctl enable-linger "$_USER"
for service in "${_USER_SERVICES[@]}"; do
  systemctl "--machine=${_USER}@.host" --user enable "$service"
done
sudo -u "$_USER" bash -l "${ENTRYPOINT}/setup/asdf.sh"
sudo -u "$_USER" bash -l "${ENTRYPOINT}/setup/user.sh"

reboot
