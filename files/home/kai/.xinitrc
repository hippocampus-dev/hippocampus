previous_default_sink=$(pactl info | awk -F': ' '/Default Sink/ {print $2}')
trap '[ -n "$previous_default_sink" ] && pactl set-default-sink "$previous_default_sink"' EXIT

export GTK_IM_MODULE=fcitx
export QT_IM_MODULE=fcitx
export XMODIFIERS="@im=fcitx"
# libXcursor otherwise sizes the cursor as the shorter screen side / 48
export XCURSOR_SIZE=32
# xdg-desktop-portal-gtk, which draws Chrome's file dialogs, is started by the systemd user manager
systemctl --user import-environment XCURSOR_SIZE
dbus-update-activation-environment XCURSOR_SIZE

fcitx5 -d

if [ -d /etc/X11/xinit/xinitrc.d ] ; then
  for f in /etc/X11/xinit/xinitrc.d/?*.sh ; do
    [ -x "$f" ] && . "$f"
  done
  unset f
fi

if xrandr --query | grep -q '^DP-0\.1 connected'; then
  xrandr --output DP-0.1 --mode 3840x2160 --rate 60 --pos 0x0 --output DP-0.8 --primary --mode 3840x2160 --rate 60 --pos 1920x2160
fi

# dunst has no display to open until X starts, so nothing raised before login was ever shown
systemctl --failed --no-legend --plain | while read -r unit _; do
  notify-send --app-name=sticky-note --hint="string:x-dunst-stack-tag:${unit}" "${unit} failed" "journalctl -b -u ${unit}"
done
systemctl --user --failed --no-legend --plain | while read -r unit _; do
  notify-send --app-name=sticky-note --hint="string:x-dunst-stack-tag:${unit}" "${unit} failed" "journalctl -b --user -u ${unit}"
done
journalctl --boot --identifier=dmeventd --output=cat | while IFS= read -r line; do
  case "$line" in
    "WARNING: Thin pool "*" is now "*"% full.")
      notify-send --urgency=critical --hint="string:x-dunst-stack-tag:${line% is now *}" "Thin pool" "$line"
      ;;
  esac
done

setsid --wait openbox-session
