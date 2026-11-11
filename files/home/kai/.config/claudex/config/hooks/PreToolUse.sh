#!/usr/bin/env bash

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

json=$(cat -)

tool_name=$(echo "$json" | jq -r '.tool_name')

[ "$tool_name" = "Bash" ] || exit 0

command=$(echo "$json" | jq -r '.tool_input.command // empty')

readonly BLOCKED_COMMANDS=(
  "gh issue create"
  "wrangler deploy"
  "wrangler secret put"
  "wrangler secret delete"
  "wrangler secret bulk"
)

for keyword_set in "${BLOCKED_COMMANDS[@]}"; do
  match=true
  read -r -a keywords <<< "$keyword_set"
  for keyword in "${keywords[@]}"; do
    if ! echo "$command" | sed "s/\\\\\"//g; s/'[^']*'//g; s/\"[^\"]*\"//g" | tr ' \t' '\n' | grep -qx "$keyword"; then
      match=false
      break
    fi
  done
  if "$match"; then
    echo "Blocked: contains '${keyword_set}'" 1>&2
    exit 2
  fi
done

exit 0
