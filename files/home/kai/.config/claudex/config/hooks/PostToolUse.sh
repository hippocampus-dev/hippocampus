#!/usr/bin/env bash

set -eo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

json=$(cat -)

tool_name=$(echo "$json" | jq -r '.tool_name')

case "$tool_name" in
  ExitPlanMode)
    plan_file=$(echo "$json" | jq -r '.tool_response.filePath')

    if grep -q '<!--.*-->' "$plan_file"; then
      jq -n --arg plan_file "$plan_file" '{
        decision: "block",
        reason: "The plan file at \($plan_file) contains <!-- --> HTML-comment feedback the user added. Read the file, treat each <!-- ... --> as feedback for the surrounding section, rewrite each section to address the comment (asking for clarification if ambiguous), save the file with all <!-- --> markers removed, summarize what changed, then continue executing the revised plan."
      }'
    fi
    ;;
  Write|Edit|MultiEdit|NotebookEdit)
    file_path=$(echo "$json" | jq -r '.tool_input.file_path')

    if [ -z "$file_path" ] || [ "$file_path" = "null" ] || [ ! -f "$file_path" ]; then
      exit 0
    fi

    make fmt 2>/dev/null || true

    sed -i --follow-symlinks 's/[[:space:]]*$//' "$file_path"

    if [ -n "$(tail -c1 "$file_path")" ]; then
      echo >> "$file_path"
    fi
    ;;
esac
