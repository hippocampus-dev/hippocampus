#!/usr/bin/env bash

set -eo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

json=$(cat -)

stop_hook_active=$(echo "$json" | jq -r '.stop_hook_active')

if [ "$stop_hook_active" = "true" ]; then
  title=$(tmux display-message -p '#{pane_title}' 2>/dev/null || echo "Agent")
  notify-send -u low -t 30000 "$title" "Stopping"
  echo '{}'
else
  echo '{"decision":"block","reason":"Verify compliance with CLAUDE.md and .claude/rules. If not fully compliant, address before stopping. If compliant, state so in one line and stop."}'
fi
