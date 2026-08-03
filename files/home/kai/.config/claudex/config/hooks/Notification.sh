#!/usr/bin/env bash

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

json=$(cat -)

message=$(echo "$json" | jq -r '.message')
title=""
if [ -n "$TMUX_PANE" ]; then
  title=$(tmux display-message -t "$TMUX_PANE" -p '#{pane_title}' 2>/dev/null || true)
fi

notify-send -u low -t 30000 "${title:-Agent}" "$message"
