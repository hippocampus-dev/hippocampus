#!/usr/bin/env bash

set -e

input=$(cat)
prompt=$(echo "$input" | jq -r '.prompt' | tr '[:upper:]' '[:lower:]')
cwd=$(echo "$input" | jq -r '.cwd')
session_id=$(echo "$input" | jq -r '.session_id' | tr '/' '_')

skills="${cwd}/.claude/skills"

[ -d "$skills" ] || exit 0

matched=()

for skill_md in "${skills}"/*/SKILL.md; do
  [ -f "$skill_md" ] || continue

  name=$(sed -n 's/^name: *//p' "$skill_md")
  keywords=$(sed -n 's/^keywords: *//p' "$skill_md" | tr '[:upper:]' '[:lower:]' | tr ',' ' ')

  for word in $keywords; do
    if [[ "$prompt" == *"$word"* ]]; then
      matched+=("$name")
      break
    fi
  done
done

new_matched=()
for skill in "${matched[@]}"; do
  flag="/tmp/claude-skill-suggested-${session_id}-${skill}"
  if [ ! -f "$flag" ]; then
    new_matched+=("$skill")
    touch "$flag"
  fi
done

if [ ${#new_matched[@]} -gt 0 ]; then
  echo ""
  echo "---"
  echo "SKILL SUGGESTIONS:"
  for skill in "${new_matched[@]}"; do
    echo "  → $skill"
  done
  echo ""
  echo "Use Skill tool to activate"
  echo "---"
fi
