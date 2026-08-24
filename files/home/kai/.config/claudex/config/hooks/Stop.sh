#!/usr/bin/env bash

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

json=$(cat -)

stop_hook_active=$(echo "$json" | jq -r '.stop_hook_active')

if [ "$stop_hook_active" != "false" ]; then
  title=$(armyknife mcp call http://127.0.0.1:47100/sse get_pane_title "{\"pane\":\"$TMUX_PANE\"}" 2>/dev/null || true)

  pane_url=""
  if [ -n "$TMUX_PANE" ]; then
    # RFC 3986 reads % as the start of a percent-encoded octet, so the pane id travels without its leading %
    pane_url="file:///tmux/pane/${TMUX_PANE#%}"$'\n\n'
  fi

  armyknife mcp call http://127.0.0.1:47101/sse notify "$(jq -nc --arg summary "${title:-Agent}" --arg body "${pane_url}Stopping" '{summary:$summary,body:$body,urgency:"low","expire-time":10}')" > /dev/null 2>&1 || true
  echo '{}'
else
  echo '{"decision":"block","reason":"Check each item before stopping. 1: if this response ends work on an instruction, it must match the final summary template in ~/.config/claudex/config/CLAUDE.summary.md exactly, including everything that file requires of `{{逐語反復}}`. 2: if any file changed, every review the task-creation list in ~/.config/claudex/config/CLAUDE.important.md requires for that kind of task must have run and you must have verified its findings yourself. 3: every factual claim must come from something you actually read this session. 4: check the response row by row against the `### Feedback` table in ~/.config/claudex/config/CLAUDE.general.md - a finding you applied and one you rejected leave no trace in the response, a finding you put to the user appears in the place and the form the output-condition table in ~/.config/claudex/config/CLAUDE.summary.md gives it, and the reviews themselves go unmentioned unless asked. 5: any other rule in CLAUDE.md or .claude/rules. Fix any gap before stopping."}'
fi
