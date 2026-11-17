#!/usr/bin/env bash

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

# files/home/kai/.config/claudex/config/settings.json already registers this hook wherever CLAUDE_CONFIG_DIR points at it
[ "$CLAUDE_CODE_REMOTE" = "true" ] || { [ "$GITHUB_ACTIONS" = "true" ] && [ "$CLAUDE_CONFIG_DIR" != ~/.config/claudex/config ]; } || exit 0

exec "$CLAUDE_PROJECT_DIR/files/home/kai/.config/claudex/config/hooks/UserPromptSubmit.sh"
