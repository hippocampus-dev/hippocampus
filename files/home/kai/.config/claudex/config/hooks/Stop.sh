#!/usr/bin/env bash

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

SCAN_LINES=400
STALE_SECONDS=1800

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
  transcript_path=$(echo "$json" | jq -r '.transcript_path // empty')

  findings=""
  if [ -f "$transcript_path" ]; then
    findings=$(tail -n "$SCAN_LINES" "$transcript_path" | timeout 10 jq -Rs -r \
      --argjson stale_seconds "$STALE_SECONDS" '
      def entry_text:
        if (.message.content? | type) == "string" then .message.content
        elif (.message.content? | type) == "array"
        then ([ .message.content[] | select(type == "object") | select(.type? == "text") | .text ] | join("\n"))
        else ""
        end;

      split("\n")
      | map(fromjson? // empty)
      | map(select(type == "object"))
      | . as $lines
      | ( [ $lines[] | (.timestamp? // "") | sub("\\.[0-9]+Z$"; "Z") | fromdateiso8601? // empty ] | max ) as $now
      | [ $lines[]
          | select(.type == "assistant")
          | ((.timestamp? // "") | sub("\\.[0-9]+Z$"; "Z") | fromdateiso8601? // null) as $at
          | (if (.message.content? | type) == "array" then .message.content else [] end)[]
          | select(type == "object")
          | select(.type? == "tool_use")
          | select(.name? == "Agent" or .name? == "Task")
          | { id: .id, name: (.input.name? // null), at: $at } ] as $launched
      | [ $lines[] | select(.type != "queue-operation") | tostring | scan("<tool-use-id>([^<]+)</tool-use-id>") | .[0] ] as $returned
      | [ $lines[] | select(.type == "user") | entry_text | scan("(?:teammate_id=|<agent-message from=)\"([^\"]+)\"") | .[0] ] as $reported
      | [ $launched
          | group_by(.name // .id)[]
          | last
          | select(.id as $id | ($returned | index($id)) == null)
          | select(.name as $name | $name == null or ($reported | index($name)) == null)
          | select($now == null or .at == null or ($now - .at) <= $stale_seconds)
          | .id ] as $outstanding
      | if ($outstanding | length) > 0
        then " Mechanical check: \($outstanding | length) subagent task(s) started in the last \(($stale_seconds / 60) | floor) minutes have not reported back yet (\($outstanding | join(", "))), so whatever they carry is not in this response."
        else ""
        end
      ' 2>/dev/null) || findings=" Mechanical check: did not run on this response."
  fi

  jq -nc --arg reason "Check each item before stopping. 1: if this response ends work on an instruction, it must match the final summary template in ~/.config/claudex/config/CLAUDE.summary.md exactly, including everything that file requires of \`{{前置き}}\` and \`{{逐語反復}}\`, and when the summary you already sent still stands unchanged, take the row its output-condition table gives that case instead of sending it again. 2: if any file changed, every review the task-creation list in ~/.config/claudex/config/CLAUDE.important.md requires for that kind of task must have run and you must have verified its findings yourself. 3: every factual claim must come from something you actually read this session. 4: check the response row by row against the \`### Feedback\` table in ~/.config/claudex/config/CLAUDE.general.md - a finding you applied and one you rejected leave no trace in the response, a finding you put to the user appears in the place and the form the output-condition table in ~/.config/claudex/config/CLAUDE.summary.md gives it, and the reviews themselves go unmentioned unless asked. 5: any other rule in CLAUDE.md or .claude/rules.${findings} Fix any gap before stopping." '{decision:"block",reason:$reason}'
fi
