#!/usr/bin/env bash

[ "$CLAUDE_CODE_REMOTE" = "true" ] || exit 0

readonly CONFIGURATION="$CLAUDE_PROJECT_DIR/files/home/kai/.config/claudex/config"

json=$(cat -)

session_source=$(echo "$json" | jq -r '.source')

cp -r "$CONFIGURATION/agents" "$CONFIGURATION/commands" "$CONFIGURATION/skills" ~/.claude/

# A resumed transcript already carries the output of the earlier SessionStart run
[ "$session_source" = "resume" ] || cat "$CONFIGURATION/CLAUDE.important.md" "$CONFIGURATION/CLAUDE.summary.md" "$CONFIGURATION/CLAUDE.general.md"
