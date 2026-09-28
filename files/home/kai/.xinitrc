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

setsid --wait openbox-session
