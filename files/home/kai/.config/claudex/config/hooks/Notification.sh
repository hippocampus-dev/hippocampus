#!/usr/bin/env bash

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

json=$(cat -)

message=$(echo "$json" | jq -r '.message')
title=$(armyknife mcp call http://127.0.0.1:47100/sse get_title "{\"pane\":\"$TMUX_PANE\"}" 2>/dev/null || true)

notify-send -u low -t 30000 "${title:-Agent}" "$message"
