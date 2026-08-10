#!/usr/bin/env bash

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

SILENCE_SECONDS=60
POLL_SECONDS=5

claudex_units() {
  # --plain drops the leading marker column systemctl adds to units that are not cleanly active
  systemctl --user list-units --type=service --plain --no-legend "claudex-*" | awk '{print $1}'
}

# Anything unreadable counts as working, since powering off is what cannot be undone
working() {
  local units unit execstart environment pane silence activity now

  now=$(date +%s)
  units=$(claudex_units) || return 0

  for unit in $units; do
    execstart=$(systemctl --user show "$unit" -p ExecStart --value) || return 0
    # A --print run writes to its caller's pipe, so its pane stays silent for the whole run
    case " $execstart " in
      *" --print "*)
        return 0
        ;;
    esac

    environment=$(systemctl --user show "$unit" -p Environment --value) || return 0
    pane=$(printf '%s\n' "$environment" | tr ' ' '\n' | awk -F= '/^TMUX_PANE=/{print $2}')
    [ -n "$pane" ] || return 0

    silence=$(tmux show-window-options -t "$pane" -v monitor-silence 2>/dev/null) || return 0
    # An agent waiting on a job zeroes this, and a never-armed window answers empty, not 0
    case "$silence" in
      '' | 0)
        return 0
        ;;
    esac

    activity=$(tmux display-message -t "$pane" -p '#{window_activity}' 2>/dev/null)
    case "$activity" in
      '' | 0 | *[!0-9]*)
        return 0
        ;;
    esac

    [ "$((now - activity))" -ge "$SILENCE_SECONDS" ] || return 0
  done

  return 1
}

units=$(claudex_units)
if [ -z "$units" ]; then
  echo "No claudex session is running" 1>&2
  exit 1
fi

echo "Waiting for every claudex session to stay silent for ${SILENCE_SECONDS} seconds (Ctrl-C to cancel)"

while working; do
  sleep "$POLL_SECONDS"
done

systemctl --no-ask-password poweroff
