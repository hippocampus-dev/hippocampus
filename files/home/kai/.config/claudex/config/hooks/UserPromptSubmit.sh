#!/usr/bin/env bash

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

readonly SUGGESTION_FLAG_PREFIX="/tmp/claude-skill-suggested"
readonly HARNESS_PREFIXES=("<task-notification>" "<system-reminder>" "<local-command" "<command-message>" "<command-name>" "<bash-input>" "<agent-message" "<teammate-message" "Stop hook feedback" "Another Claude session")

json=$(cat -)
prompt=$(echo "$json" | jq -r '.prompt' | tr '[:upper:]' '[:lower:]')
current_directory=$(echo "$json" | jq -r '.cwd')
session_id=$(echo "$json" | jq -r '.session_id' | tr '/' '_')

for prefix in "${HARNESS_PREFIXES[@]}"; do
  case "$prompt" in
    "${prefix,,}"*)
      exit 0
      ;;
  esac
done

skills="${current_directory}/.claude/skills"

[ -d "$skills" ] || exit 0

matches() {
  local word="$1"
  local before='(^|[^a-z0-9])'
  local after='s?([^a-z0-9]|$)'

  # Japanese carries no word delimiter, so a keyword with any character outside [a-z0-9._-] can only be matched as a substring
  if [[ "$word" =~ [^a-z0-9._-] ]]; then
    case "$prompt" in
      *"$word"*)
        return 0
        ;;
    esac
    return 1
  fi

  [[ "$word" =~ ^[a-z0-9] ]] || before=""
  [[ "$word" =~ [a-z0-9]$ ]] || after=""
  [[ "$prompt" =~ ${before}"$word"${after} ]] && return 0
  return 1
}

matched=()

for skill_md in "$skills"/*/SKILL.md; do
  [ -f "$skill_md" ] || continue

  name=$(sed -n 's/^name: *//p' "$skill_md")
  keywords=($(sed -n 's/^keywords: *//p' "$skill_md" | tr '[:upper:]' '[:lower:]' | tr ',' ' '))

  for word in "${keywords[@]}"; do
    if matches "$word"; then
      matched+=("$name")
      break
    fi
  done
done

new_matched=()
for skill in "${matched[@]}"; do
  flag="${SUGGESTION_FLAG_PREFIX}-${session_id}-${skill}"
  if [ ! -f "$flag" ]; then
    new_matched+=("$skill")
    touch "$flag"
  fi
done

if [ "${#new_matched[@]}" -gt 0 ]; then
  echo "<system-reminder>"
  echo "SKILL SUGGESTIONS:"
  for skill in "${new_matched[@]}"; do
    echo "  → $skill"
  done
  echo ""
  echo "Use Skill tool to activate if applicable."
  echo "</system-reminder>"
fi
