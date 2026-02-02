#!/usr/bin/env bash

set -eo pipefail

json=$(cat -)

stop_hook_active=$(echo "$json" | jq -r '.stop_hook_active')

if [ "$stop_hook_active" = "true" ]; then
  notify-send -u low -t 30000 Claude "Stopping Claude"
  echo '{}'
else
  echo '{"decision":"block","reason":"Verify compliance with CLAUDE.md and .claude/rules. If not fully compliant, address before stopping. If compliant, state so in one line and stop."}'
fi
