trap '[ -f "${XDG_RUNTIME_DIR:-/tmp}/PREVIOUS_DEFAULT_SINK" ] && pactl set-default-sink $(cat "${XDG_RUNTIME_DIR:-/tmp}/PREVIOUS_DEFAULT_SINK")' EXIT

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

openbox-session
