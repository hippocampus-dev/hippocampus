previous_default_sink=$(pactl info | awk -F': ' '/Default Sink/ {print $2}')
trap '[ -n "$previous_default_sink" ] && pactl set-default-sink "$previous_default_sink"' EXIT

export GTK_IM_MODULE=fcitx
export QT_IM_MODULE=fcitx
export XMODIFIERS="@im=fcitx"

fcitx5 -d

if [ -d /etc/X11/xinit/xinitrc.d ] ; then
  for f in /etc/X11/xinit/xinitrc.d/?*.sh ; do
    [ -x "$f" ] && . "$f"
  done
  unset f
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
