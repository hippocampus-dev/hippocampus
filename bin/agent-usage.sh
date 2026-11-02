#!/usr/bin/env bash

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

ENTRYPOINT=$(cd "$(dirname "${BASH_SOURCE[0]}")"; pwd)
TRANSCRIPTS="${HOME}/.config/claudex/config/projects"
SCAN_DAYS=30
REPLAY_LIMIT=100
REPLAY_PER_SESSION=5

repository=$(cd "${ENTRYPOINT}/.."; pwd)
configuration="${repository}/files/home/kai/.config/claudex/config"
stop_hook="${configuration}/hooks/Stop.sh"

transcripts=()
while IFS= read -r file; do
  transcripts+=("$file")
done < <(find -L "$TRANSCRIPTS" -name "*.jsonl" -newermt "${SCAN_DAYS} days ago" -printf "%T@\t%p\n" | sort -rn | cut -f 2)

if [ "${#transcripts[@]}" -eq 0 ]; then
  echo "No transcript under ${TRANSCRIPTS} is newer than ${SCAN_DAYS} days, so nothing can be measured."
  exit 0
fi

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

uses="${work}/uses"
delivered_checks="${work}/delivered"
replayed_checks="${work}/replayed"
: > "$delivered_checks"
: > "$replayed_checks"

function report() {
  local kind="$1"
  local name="$2"
  local path="$3"
  local alias="${4:-}"
  local added
  added=$(git -C "$repository" log --follow --diff-filter=A --format=%ad --date=short -- "$path" | tail -n 1)
  printf "  %-42s %s  %s\n" \
    "$name" \
    "$(awk -F'\t' -v kind="$kind" -v name="$name" -v alias="$alias" '
      ($1 == kind && $2 == name) || (alias != "" && $1 == "command" && $2 == alias) {
        count++
        if (!seen[$3]++) sessions++
        if ($4 > last) last = $4
      }
      END { printf "%5d uses  %4d sessions  last %s", count, sessions, (last == "" ? "never" : last) }
    ' "$uses")" \
    "added ${added:-uncommitted}"
}

function mechanical_checks() {
  awk '{
    at = index($0, "Mechanical check: ")
    if (at == 0) next
    text = substr($0, at)
    sub(/ Fix any gap.*/, "", text)
    gsub(/toolu_[A-Za-z0-9]+/, "ID", text)
    gsub(/[0-9]+/, "N", text)
    print substr(text, 1, 160)
  }'
}

jq -r '
  (input_filename | sub(".*/"; "") | sub("\\.jsonl$"; "")) as $session
  | ((.timestamp? // "") | sub("T.*"; "")) as $day
  | (if (.message.content? | type) == "array" then .message.content else [] end) as $blocks
  | ( [ $blocks[]
        | select(type == "object")
        | select(.type? == "tool_use")
        | if .name? == "Agent" or .name? == "Task" then "agent\t" + (.input.subagent_type? // "")
          elif .name? == "Skill" then "skill\t" + (.input.skill? // "")
          else empty
          end ]
      + [ tostring | scan("<command-name>(/[^<]+)</command-name>") | "command\t" + .[0] ] )[]
  | . + "\t" + $session + "\t" + $day
' "${transcripts[@]}" 2> /dev/null > "$uses"

echo "Scanned ${#transcripts[@]} transcript(s) written in the last ${SCAN_DAYS} days under ${TRANSCRIPTS}."
echo

echo "Subagents (${configuration}/agents/)"
while IFS= read -r path; do
  report agent "$(awk -F': ' '/^name:/ { print $2; exit }' "$path")" "$path"
done < <(find -L "${configuration}/agents" -name "*.md" ! -name "AGENTS.md" | sort)
echo

echo "Commands (${configuration}/commands/)"
while IFS= read -r path; do
  name=${path#"${configuration}/commands/"}
  name=${name%.md}
  report command "/${name//\//:}" "$path"
done < <(find -L "${configuration}/commands" -name "*.md" ! -name "AGENTS.md" | sort)
echo

echo "Skills (${repository}/.claude/skills/)"
while IFS= read -r path; do
  report skill "$(basename "$path")" "$path" "/$(basename "$path")"
done < <(find -L "${repository}/.claude/skills" -mindepth 1 -maxdepth 1 -type d | sort)
echo

marker=$(echo '{"stop_hook_active":false}' | bash "$stop_hook" | jq -re '.reason' | cut -c 1-40)

stops=$(jq -r --arg marker "$marker" '
  select(.type == "user")
  | select(.message.content | type == "string" and startswith("Stop hook feedback"))
  | { file: input_filename, line: input_line_number, text: tostring }
  | select(.text | contains($marker))
  | [ .file, (.line | tostring), (.text | split($marker) | .[1] | [ scan("Mechanical check: .*?(?= Fix any gap)") ] | first // "") ]
  | @tsv
' "${transcripts[@]}" 2> /dev/null)

stop_points=0
reported=0
replayed=0
session=""
from_session=0
while IFS=$'\t' read -r file line delivered; do
  [ -n "$file" ] || continue
  stop_points=$((stop_points + 1))
  [ -z "$delivered" ] || reported=$((reported + 1))
  if [ "$file" != "$session" ]; then
    session="$file"
    from_session=0
  fi
  [ "$replayed" -lt "$REPLAY_LIMIT" ] && [ "$from_session" -lt "$REPLAY_PER_SESSION" ] || continue
  from_session=$((from_session + 1))
  printf "%s\n" "$delivered" >> "$delivered_checks"
  head -n "$((line - 1))" "$file" > "${work}/transcript"
  echo "{\"stop_hook_active\":false,\"transcript_path\":\"${work}/transcript\"}" | bash "$stop_hook" | jq -re '.reason' >> "$replayed_checks"
  replayed=$((replayed + 1))
done <<< "$stops"

echo "Stop hook checks (${stop_hook})"
if [ "$stop_points" -eq 0 ]; then
  echo "  No stop point matched. The marker this script takes from the hook's own reason no longer appears in any transcript, so nothing below could have been measured."
else
  echo "  ${stop_points} recorded stop point(s), ${reported} of which were handed a check."
  echo "  Both lists below cover the same sample: up to ${REPLAY_PER_SESSION} stop point(s) from each of the most recently written sessions, ${replayed} in total."
  echo
  echo "  Delivered - what the hook version live at the time actually said:"
  mechanical_checks < "$delivered_checks" | LC_ALL=C sort | uniq -c | sort -rn | sed 's/^/  /'
  echo "  Replayed - what the current hook says at those same stop points:"
  mechanical_checks < "$replayed_checks" | LC_ALL=C sort | uniq -c | sort -rn | sed 's/^/  /'
fi
echo
echo "  Checks the current hook can emit:"
awk '{
  at = index($0, "Mechanical check: ")
  if (at == 0) next
  text = substr($0, at)
  sub(/[[:space:]]*$/, "", text)
  sub(/"$/, "", text)
  printf "    %s\n", text
}' "$stop_hook"
echo
echo "Not covered: PreToolUse, PostToolUse, UserPromptSubmit, Notification and SessionEnd."
echo "  Counting their firings would need each one's output strings copied into this script, and Notification.sh and SessionEnd.sh write nothing a transcript records."
