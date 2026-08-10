#!/usr/bin/env bash

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

PROJECTS_DIRECTORY=~/.config/claudex/config/projects
DAYS=7

if [ ! -d "$PROJECTS_DIRECTORY" ]; then
  echo "No transcripts under ${PROJECTS_DIRECTORY}" 1>&2
  exit 1
fi

pending_sessions() {
  local transcript asked answered unanswered session

  while IFS= read -r transcript; do
    # jq stops at a malformed line, and errexit is off inside the substitution calling this
    if ! asked=$(jq -r '(.message.content // [])[]? | select(.type == "tool_use" and .name == "AskUserQuestion") | .id' "$transcript" 2>/dev/null | LC_ALL=C sort -u); then
      echo "cannot read ${transcript}" 1>&2
      continue
    fi

    [ -n "$asked" ] || continue

    if ! answered=$(jq -r '(.message.content // [])[]? | select(.type == "tool_result") | .tool_use_id' "$transcript" 2>/dev/null | LC_ALL=C sort -u); then
      echo "cannot read ${transcript}" 1>&2
      continue
    fi
    unanswered=$(LC_ALL=C comm -23 <(printf '%s\n' "$asked") <(printf '%s\n' "$answered") | awk 'NR==1')

    [ -n "$unanswered" ] || continue

    # The name reaches a shell through send-keys and the sandbox can write into this tree
    session=$(basename "$transcript" .jsonl)
    if [[ ! "$session" =~ ^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$ ]]; then
      continue
    fi

    # claude --resume scopes to the project of the cwd it started in, not where it ended
    printf '%s\t%s\t%s\n' \
      "$session" \
      "$(jq -r 'select(.cwd) | .cwd' "$transcript" | awk 'NR==1')" \
      "$(jq -r --arg id "$unanswered" '(.message.content // [])[]? | select(.id == $id) | .input.questions[0].question' "$transcript" | awk 'NR==1')"
  done < <(find "$PROJECTS_DIRECTORY" -mindepth 2 -maxdepth 2 -name '*.jsonl' -type f -mtime -"$DAYS")
}

sessions=$(pending_sessions)

if [ -z "$sessions" ]; then
  echo "No claudex session is waiting on a question"
  exit 0
fi

running_units=$(systemctl --user list-units --type=service --plain --no-legend "claudex-*")
if [ -n "$running_units" ]; then
  echo "A claudex session is still running - answer or finish it before resuming" 1>&2
  exit 1
fi

if [ -z "$TMUX_PANE" ]; then
  echo "Run this inside tmux so the windows land in a session you can see" 1>&2
  exit 1
fi

if ! own_session=$(tmux list-panes -t "$TMUX_PANE" -F "#{session_name}" | awk 'NR==1'); then
  echo "No tmux pane to open the sessions from" 1>&2
  exit 1
fi

while IFS=$'\t' read -r session directory question; do
  # tmux falls back to the home directory rather than failing on a missing one
  if [ ! -d "$directory" ]; then
    echo "${session}  ${directory} is gone, skipping" 1>&2
    continue
  fi

  pane=$(tmux new-window -d -t "${own_session}:" -c "$directory" -P -F "#{pane_id}")
  tmux send-keys -t "$pane" "claudex --resume ${session}" C-m
  printf '%s  %s  %s\n  %s\n' "$session" "$directory" "$pane" "$question"
done <<< "$sessions"
