#!/usr/bin/env bash

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

json=$(cat -)

is_antigravity=$(echo "$json" | jq -r 'has("toolCall")')
tool_name=$(echo "$json" | jq -r '.tool_name // .toolCall.name // empty')

case "$tool_name" in
  Bash|run_command|run_shell_command) ;;
  *)
    [ "$is_antigravity" = "true" ] && echo '{"decision":"allow"}'
    exit 0
    ;;
esac

command=$(echo "$json" | jq -r '.tool_input.command // .toolCall.args.CommandLine // .toolCall.args.command // empty')

readonly BLOCKED_COMMANDS=(
  "gh issue create"
  "wrangler deploy"
  "wrangler secret put"
  "wrangler secret delete"
  "wrangler secret bulk"
)

for keyword_set in "${BLOCKED_COMMANDS[@]}"; do
  # .github/workflows/claude-auto-fix-ci.yaml prompts its agent to file an issue
  if [ "$keyword_set" = "gh issue create" ] && [ "$GITHUB_ACTIONS" = "true" ]; then
    continue
  fi
  match=true
  read -r -a keywords <<< "$keyword_set"
  for keyword in "${keywords[@]}"; do
    if ! echo "$command" | sed "s/\\\\\"//g; s/'[^']*'//g; s/\"[^\"]*\"//g" | tr ' \t' '\n' | grep -qx "$keyword"; then
      match=false
      break
    fi
  done
  if "$match"; then
    if [ "$is_antigravity" = "true" ]; then
      jq -nc --arg reason "Blocked: contains '${keyword_set}'" '{decision:"deny",reason:$reason}'
      exit 0
    fi
    echo "Blocked: contains '${keyword_set}'" 1>&2
    exit 2
  fi
done

[ "$is_antigravity" = "true" ] && echo '{"decision":"allow"}'
exit 0
