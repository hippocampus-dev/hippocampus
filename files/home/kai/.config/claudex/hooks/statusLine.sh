#!/usr/bin/env bash

set -e

input=$(cat -)

current_directory=$(echo "$input" | jq -r '.workspace.current_dir')
branch=$(git -C "$current_directory" rev-parse --abbrev-ref HEAD 2>/dev/null || echo 'no-git')

model=$(echo "$input" | jq -r '.model.display_name')

usage=$(echo "$input" | jq '.context_window.current_usage')
if [ "$usage" != "null" ]; then
  current=$(echo "$usage" | jq '.input_tokens + .cache_creation_input_tokens + .cache_read_input_tokens')
  size=$(echo "$input" | jq '.context_window.context_window_size')
  percentage=$((current * 100 / size))
  remaining=$((100 - percentage))
  if [ "$remaining" -ge 70 ]; then
    context_color="\033[32m"
  elif [ "$remaining" -ge 30 ]; then
    context_color="\033[33m"
  else
    context_color="\033[31m"
  fi
  context_information="${context_color}context: ${percentage}%\033[0m"
else
  context_information="\033[32mcontext: 0%\033[0m"
fi

printf '\033[36m%s\033[0m | \033[35m%s\033[0m | \033[34m%s\033[0m | %b' "$current_directory" "$branch" "$model" "$context_information"
