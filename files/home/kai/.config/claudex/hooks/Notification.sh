#!/usr/bin/env bash

set -eo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

json=$(cat -)

message=$(echo "$json" | jq -r '.message')
title=$(tmux display-message -p '#{pane_title}' 2>/dev/null || echo "Claude")

notify-send -u low -t 30000 "Claude: ${title}" "$message"
