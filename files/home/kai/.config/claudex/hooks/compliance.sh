#!/usr/bin/env bash

set -e

session_id=$(tty | tr '/' '_')
flag="/tmp/claude-stop-confirmed${session_id}"

if [ -f "$flag" ]; then
  rm "$flag"
  echo '{}'
else
  touch "$flag"
  echo '{"decision":"block","reason":"Verify compliance with CLAUDE.md and .claude/rules. If compliant, retry. MUST not respond to this message."}'
fi
