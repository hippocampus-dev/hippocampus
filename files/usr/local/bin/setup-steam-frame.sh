#!/usr/bin/env bash

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

readonly GITHUB_USER=kaidotio
readonly TARGET=steamos@frame

version=$(awk '$1 == "hippocampus" {print $2}' ~/.tool-versions)
server="$(uname -n).local"
public_keys=$(curl -fsSL --retry 5 --retry-all-errors "https://github.com/${GITHUB_USER}.keys")

work=$(mktemp -d)
cleanup() {
  if [ -S "${work}/ssh.socket" ]; then
    ssh -S "${work}/ssh.socket" -O exit "$TARGET" 2> /dev/null || true
  fi
  rm -rf "$work"
}
trap cleanup EXIT
ssh -f -N -M -S "${work}/ssh.socket" "$TARGET"

echo "=== authorized_keys: ${GITHUB_USER}.keys"
ssh -S "${work}/ssh.socket" "$TARGET" 'set -e; umask 077; mkdir -p ~/.ssh; touch ~/.ssh/authorized_keys; cat ~/.ssh/authorized_keys - | awk "NF && !seen[\$0]++" > ~/.ssh/authorized_keys.part; mv ~/.ssh/authorized_keys.part ~/.ssh/authorized_keys' <<< "$public_keys"

arch=$(ssh -S "${work}/ssh.socket" "$TARGET" uname -m | sed 's/^x86_64$/amd64/; s/^aarch64$/arm64/')
echo "=== teleput: ${version} for ${arch}"
gh release download "v${version}" --repo "${GITHUB_USER}/hippocampus" --pattern "teleput_${version}_linux_${arch}" --output "${work}/teleput"
# A rename keeps a running client from failing the write with ETXTBSY
ssh -S "${work}/ssh.socket" "$TARGET" 'set -e; mkdir -p ~/.local/bin; cat > ~/.local/bin/teleput.part; chmod +x ~/.local/bin/teleput.part; mv ~/.local/bin/teleput.part ~/.local/bin/teleput' < "${work}/teleput"

echo "=== peer.sha256: pairing over SSH"
server_fingerprint=$(teleput fingerprint)
client_fingerprint=$(ssh -S "${work}/ssh.socket" "$TARGET" '~/.local/bin/teleput fingerprint')
ssh -S "${work}/ssh.socket" "$TARGET" 'set -e; umask 077; mkdir -p "${XDG_CONFIG_HOME:-${HOME}/.config}/teleput"; cat > "${XDG_CONFIG_HOME:-${HOME}/.config}/teleput/peer.sha256"' <<< "$server_fingerprint"
(
  umask 077
  mkdir -p "${XDG_CONFIG_HOME:-${HOME}/.config}/teleput"
  printf '%s\n' "$client_fingerprint" > "${XDG_CONFIG_HOME:-${HOME}/.config}/teleput/peer.right.sha256"
)
# teleput server reads peer.right.sha256 only at startup
systemctl --user try-restart teleput-server.service

echo "=== teleput-client.sh: ${server}:24801"
# /usr/bin/steamos-nested-desktop removes the headset desktop's XDG_RUNTIME_DIR on exit and recreates wayland-0 at the same path, while its dbus-run-session bus gets a new path every time
ssh -S "${work}/ssh.socket" "$TARGET" 'set -e; cat > ~/.local/bin/teleput-client.sh.part; chmod +x ~/.local/bin/teleput-client.sh.part; mv ~/.local/bin/teleput-client.sh.part ~/.local/bin/teleput-client.sh' <<EOS
#!/bin/sh
bus=\${DBUS_SESSION_BUS_ADDRESS#unix:path=}
bus=\${bus%%,*}
while [ -S "\${XDG_RUNTIME_DIR}/\${WAYLAND_DISPLAY}" ] && [ -S "\$bus" ]; do
  ~/.local/bin/teleput client ${server}:24801
  sleep 5
done
EOS
# The headset desktop runs under dbus-run-session from /usr/bin/steamos-nested-desktop rather than as a systemd unit, so only its own autostart sees its session bus
ssh -S "${work}/ssh.socket" "$TARGET" 'set -e; mkdir -p ~/.config/autostart; printf "[Desktop Entry]\nType=Application\nName=teleput client\nExec=%s/.local/bin/teleput-client.sh\n" "$HOME" > ~/.config/autostart/teleput-client.desktop'

echo "=== kwin cursor"
# The headset desktop's nested KWin leaves its cursor to the host's wl_pointer: https://invent.kde.org/plasma/kwin/-/blob/v6.2.5/src/backends/wayland/wayland_output.cpp
# The host's pointer never moves with teleput
# The headset's user manager carries XCURSOR_SIZE=256
# KWin prefers XCURSOR_THEME and XCURSOR_SIZE over kcminputrc: https://invent.kde.org/plasma/kwin/-/blob/v6.2.5/src/cursor.cpp
ssh -S "${work}/ssh.socket" "$TARGET" 'set -e; kwriteconfig6 --file kwinrc --group Plugins --key hidecursorEnabled true; kwriteconfig6 --file kwinrc --group Effect-hidecursor --key InactivityDuration 3'
# /usr/bin/steamos-nested-desktop inherits its environment from steam.service when launched
ssh -S "${work}/ssh.socket" "$TARGET" 'set -e; mkdir -p ~/.config/systemd/user/steam.service.d; cd ~/.config/systemd/user/steam.service.d; printf "[Service]\nEnvironment=KWIN_FORCE_SW_CURSOR=1 XCURSOR_SIZE=24\n" > kwin-cursor.conf.part; if cmp -s kwin-cursor.conf.part kwin-cursor.conf; then rm kwin-cursor.conf.part; else mv kwin-cursor.conf.part kwin-cursor.conf; systemctl --user daemon-reload; systemctl --user try-restart steam.service; fi'

echo "=== ibus trigger: Control+space"
# Matches [Hotkey/TriggerKeys] in files/home/kai/.config/fcitx5/config
# The headset desktop's dconf-service overwrites ~/.config/dconf/user from its own cache: https://gitlab.gnome.org/GNOME/dconf/-/blob/main/service/dconf-writer.c
ssh -S "${work}/ssh.socket" "$TARGET" 'cat > ~/.config/autostart/ibus-trigger.desktop' <<EOS
[Desktop Entry]
Type=Application
Name=ibus trigger
Exec=gsettings set org.freedesktop.ibus.general.hotkey triggers "['<Control>space']"
EOS
