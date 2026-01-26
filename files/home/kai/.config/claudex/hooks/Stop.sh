#!/usr/bin/env bash

set -e

session_id=$(tty | tr '/' '_')
flag="/tmp/claude-stop-confirmed-${session_id}"

if [ -f "$flag" ]; then
  rm "$flag"
  notify-send -u low -t 30000 Claude "Stopping Claude"
  echo '{}'
else
  touch "$flag"
  echo '{"decision":"block","reason":"Verify compliance with CLAUDE.md and .claude/rules, .claude/skills. If compliant, retry. MUST not respond to this message."}'
fi
