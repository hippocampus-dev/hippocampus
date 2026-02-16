#!/usr/bin/env bash

set -eo pipefail

json=$(cat -)

tool_name=$(echo "$json" | jq -r '.tool_name')

[ "$tool_name" = "Bash" ] || exit 0

command=$(echo "$json" | jq -r '.tool_input.command // empty')

blocked_commands=(
  "gh issue create"
)

for keyword_set in "${blocked_commands[@]}"; do
  match=true
  for keyword in $keyword_set; do
    if [[ "$command" != *"$keyword"* ]]; then
      match=false
      break
    fi
  done
  if $match; then
    echo "Blocked: contains '$keyword_set'" >&2
    exit 2
  fi
done

exit 0
