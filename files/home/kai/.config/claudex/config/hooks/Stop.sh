#!/usr/bin/env bash

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

readonly SCAN_LINES=400
readonly STALE_SECONDS=1800

json=$(cat -)

# The caller reads only the agent's last reply, which a block would turn into the reply to this check
if [ -n "$LAST_REPLY_ONLY" ]; then
  echo "{}"
  exit 0
fi

is_antigravity=$(echo "$json" | jq -r 'has("continuationActive") or has("conversationId")')

if [ "$is_antigravity" = "true" ]; then
  termination_reason=$(echo "$json" | jq -r '.terminationReason // empty')
  fully_idle=$(echo "$json" | jq -r 'if has("fullyIdle") then .fullyIdle else true end')
  if [ "$fully_idle" != "true" ] || { [ -n "$termination_reason" ] && [ "$termination_reason" != "NO_TOOL_CALL" ]; }; then
    echo "{}"
    exit 0
  fi
fi

stop_hook_active=$(echo "$json" | jq -r '
  if has("stop_hook_active") then .stop_hook_active
  elif .continuationActive == true or .continuation_active == true or ((.executionNum // 0) > 0) then true
  elif has("continuationActive") or has("continuation_active") or has("executionNum") then false
  else null
  end')

transcript_path=$(echo "$json" | jq -r '.transcript_path // .transcriptPath // empty')

# Antigravity can send continuationActive=false even after a Stop hook continuation
if [ "$is_antigravity" = "true" ] && [ "$stop_hook_active" = "false" ] && [ -f "$transcript_path" ]; then
  already_blocked=$(tail -n "$SCAN_LINES" "$transcript_path" | jq -Rs -r '
    split("\n")
    | map(fromjson? // empty)
    | map(select(type == "object"))
    | (to_entries | map(select(.value.type == "USER_INPUT")) | last | .key // -1) as $last_user
    | [ to_entries[] | select(.key > $last_user) | .value | select(.type == "SYSTEM_MESSAGE" and ((.content? // "") | tostring | contains("Stop hook blocked termination:"))) ]
    | length > 0
  ' 2>/dev/null || echo "false")
  if [ "$already_blocked" = "true" ]; then
    stop_hook_active="true"
  fi
fi

if [ "$stop_hook_active" != "false" ]; then
  title=$(armyknife mcp call http://127.0.0.1:47100/sse get_pane_title "{\"pane\":\"${TMUX_PANE}\"}" 2>/dev/null || true)

  pane_url=""
  if [ -n "$TMUX_PANE" ]; then
    # RFC 3986 reads % as the start of a percent-encoded octet, so the pane id travels without its leading %
    pane_url="file:///tmux/pane/${TMUX_PANE#%}"$'\n\n'
  fi

  armyknife mcp call http://127.0.0.1:47101/sse notify "$(jq -nc --arg summary "${title:-Agent}" --arg body "${pane_url}Stopping" '{summary:$summary,body:$body,urgency:"low","expire-time":10}')" > /dev/null 2>&1 || true
  echo "{}"
else
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

  reason="Check each item before stopping. 1: if this response ends work on an instruction, it must match the final summary template under \`## Final Summary\` in CLAUDE.summary.md exactly, including everything that file requires of \`{{前置き}}\` and \`{{逐語反復}}\`, and when the summary you already sent still stands unchanged, take the row its output-condition table gives that case instead of sending it again. 2: if any file changed, every review the task-creation list naming \`準拠\`, \`棚卸し\` and \`並列: レビュー\` in CLAUDE.important.md requires for that kind of task must have run and you must have verified its findings yourself. 3: every factual claim must come from something you actually read this session, and one you could check by reading a file, reproducing it or fetching documentation must have been checked before answering rather than left as possible or unconfirmed for the user to settle, as the factual-claim item of \`## Response Policy\` in CLAUDE.general.md requires - only a claim with no means to check it stays unconfirmed, saying so and why. 4: check the response row by row against the \`### Feedback\` table in CLAUDE.general.md - a finding you applied and one you rejected leave no trace in the response, a finding you put to the user appears in the place and the form the output-condition table under \`## Final Summary\` in CLAUDE.summary.md gives it, and the reviews themselves go unmentioned unless asked. 5: any other rule in CLAUDE.md or .claude/rules.${findings} Fix any gap before stopping."

  if [ "$CLAUDECODE" = "1" ]; then
    jq -nc --arg reason "$reason" '{hookSpecificOutput:{hookEventName:"Stop",additionalContext:$reason}}'
  elif [ "$is_antigravity" = "true" ]; then
    jq -nc --arg reason "$reason" '{decision:"continue",reason:$reason}'
  else
    jq -nc --arg reason "$reason" '{decision:"block",reason:$reason}'
  fi
fi
