#!/usr/bin/env bash

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

json=$(cat -)

is_antigravity=$(echo "$json" | jq -r 'has("toolCall")')
tool_name=$(echo "$json" | jq -r '.tool_name // .toolCall.name // empty')

format_file() {
  local file_path="$1"

  if [ -z "$file_path" ] || [ "$file_path" = "null" ] || [ ! -f "$file_path" ]; then
    return 0
  fi

  make fmt 2>/dev/null || true

  sed -i --follow-symlinks 's/[[:space:]]*$//' "$file_path"

  if [ -n "$(tail -c1 "$file_path")" ]; then
    echo >> "$file_path"
  fi
}

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
  Write|Edit|MultiEdit|NotebookEdit|write_file|edit_file|write_to_file|replace_file_content|multi_replace_file_content|replace)
    file_path=$(echo "$json" | jq -r '.tool_input.file_path // .toolCall.args.Path // .toolCall.args.TargetFile // .toolCall.args.file_path // empty')
    format_file "$file_path"
    ;;
  apply_patch)
    current_directory=$(echo "$json" | jq -r '.cwd // empty')
    patch=$(echo "$json" | jq -r '.tool_input | if type == "string" then . else (.input // .patch // .command // empty) end')
    while IFS= read -r file_path; do
      [ -n "$file_path" ] || continue
      if [ -n "$current_directory" ] && [ "${file_path#/}" = "$file_path" ]; then
        file_path="${current_directory}/${file_path}"
      fi
      format_file "$file_path"
    done < <(printf '%s\n' "$patch" | sed -n -E 's/^\*\*\* (Update|Add) File: (.+)$/\2/p')
    ;;
esac

[ "$is_antigravity" = "true" ] && echo "{}"
exit 0
