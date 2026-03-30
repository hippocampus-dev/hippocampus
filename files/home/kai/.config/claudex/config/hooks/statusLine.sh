#!/usr/bin/env bash

set -eo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

input=$(cat -)

current_directory=$(echo "$input" | jq -r '.workspace.current_dir')
branch=$(git -C "$current_directory" rev-parse --abbrev-ref HEAD 2>/dev/null || echo "no-git")

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

now=$(date +%s)

five_hour=$(echo "$input" | jq -r '.rate_limits.five_hour.used_percentage | . * 10 | round / 10')
five_hour_int=${five_hour%.*}
if [ "$five_hour_int" -lt 30 ]; then
  five_hour_color="\033[32m"
elif [ "$five_hour_int" -lt 70 ]; then
  five_hour_color="\033[33m"
else
  five_hour_color="\033[31m"
fi
five_hour_resets_at=$(echo "$input" | jq -r '.rate_limits.five_hour.resets_at // empty')
five_hour_reset=""
if [ -n "$five_hour_resets_at" ]; then
  five_hour_remaining=$((five_hour_resets_at - now))
  if [ "$five_hour_remaining" -gt 0 ]; then
    five_hour_reset=" ($(( five_hour_remaining / 3600 ))h$(( (five_hour_remaining % 3600) / 60 ))m)"
  fi
fi

seven_day=$(echo "$input" | jq -r '.rate_limits.seven_day.used_percentage | . * 10 | round / 10')
seven_day_int=${seven_day%.*}
if [ "$seven_day_int" -lt 30 ]; then
  seven_day_color="\033[32m"
elif [ "$seven_day_int" -lt 70 ]; then
  seven_day_color="\033[33m"
else
  seven_day_color="\033[31m"
fi
seven_day_resets_at=$(echo "$input" | jq -r '.rate_limits.seven_day.resets_at // empty')
seven_day_reset=""
if [ -n "$seven_day_resets_at" ]; then
  seven_day_remaining=$((seven_day_resets_at - now))
  if [ "$seven_day_remaining" -gt 0 ]; then
    seven_day_reset=" ($(( seven_day_remaining / 86400 ))d$(( (seven_day_remaining % 86400) / 3600 ))h)"
  fi
fi

rate_limit_information="${five_hour_color}5h: ${five_hour}%${five_hour_reset}\033[0m | ${seven_day_color}7d: ${seven_day}%${seven_day_reset}\033[0m"

printf '\033[36m%s\033[0m | \033[35m%s\033[0m | \033[34m%s\033[0m | %b | %b' "$current_directory" "$branch" "$model" "$context_information" "$rate_limit_information"
