#!/usr/bin/env bash

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

json=$(cat -)

stop_hook_active=$(echo "$json" | jq -r '.stop_hook_active')

if [ "$stop_hook_active" = "true" ]; then
  title=""
  if [ -n "$TMUX_PANE" ]; then
    title=$(tmux display-message -t "$TMUX_PANE" -p '#{pane_title}' 2>/dev/null || true)
  fi
  notify-send -u low -t 30000 "${title:-Agent}" "Stopping"
  echo '{}'
else
  echo '{"decision":"block","reason":"Check each item before stopping. 1: if this response ends work on an instruction, it must match the final summary template in ~/.config/claudex/config/CLAUDE.summary.md exactly; reporting a background review counts as ending work. 2: if any file changed, every review the task-creation list in ~/.config/claudex/config/CLAUDE.important.md requires for that kind of task must have run and you must have verified its findings yourself. 3: every factual claim must come from something you actually read this session. 4: any other rule in CLAUDE.md or .claude/rules. Fix any gap before stopping."}'
fi
