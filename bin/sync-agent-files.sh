#!/usr/bin/env bash

set -Eeo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

# files/ outputs must stay world-readable under the sandbox's UMask=0077
umask 022

usage() {
  cat <<'EOS'
Usage:
   sync-agent-files.sh

Convert Claude Code configurations for Codex, Gemini, Antigravity, and opencode CLI.

Project-level synchronization:
  Feature     Claude Code  Codex          Gemini       opencode     Antigravity
  Skills      Source       Converted      Converted    Native       Converted
  Commands    Source       Converted [2]  Converted    Converted    Converted [2]
  Agents      Source       Partial [1]    Converted    Converted    Partial [6]
  Hooks       Source       Partial [3]    Not synced   Not synced   Partial [3]
  Path rules  Source       Partial [5]    Partial [5]  Partial [5]  Converted

Global synchronization:
  Feature     Claude Code  Codex          Gemini       opencode     Antigravity
  Skills      Source       Converted      Converted    Converted    Converted
  Commands    Source       Converted [2]  Converted    Converted    Converted [2]
  Agents      Source       Partial [1]    Converted    Converted    Partial [6]
  Hooks       Source       Partial [3]    Partial [3]  Manual [4]   Partial [3]
  Path rules  Source       Not synced     Not synced   Not synced   Not synced

Notes:
  [1] Codex has no per-agent tool allow-list; agents without write or removal tools use a read-only sandbox.
  [2] Codex and Antigravity load commands as Agent Skills under .agents/skills/
      (project) and ~/.agents/skills/ (global; Antigravity uses ~/.gemini/config/skills.json),
      joining nested paths with "-" (e.g. git/stage.md -> git-stage).
  [3] Supported hook events and tool matchers are translated; hook payloads remain tool-specific.
  [4] Hook plugins are hand-maintained and are not synchronized by this script.
  [5] An exact directory/** matcher becomes directory-scoped instructions. Other
      globs become conditional references to the source rule because AGENTS.md
      does not enforce path globs.
  [6] Antigravity native tool names come from the built-in migrate-agent-settings
      skill; the todo tools, Skill, NotebookEdit and MCP tools have no counterpart
      there and are dropped from the allow-list. File cleanup uses run_command
      with the parent's command permissions and retains its rm-only instructions.

Project-level:
  - Skills:   .claude/skills/   -> .gemini/skills/, .agents/skills/ (Codex and Antigravity both read .agents/skills/; opencode reads .claude/skills/ natively)
  - Commands: .claude/commands/ -> .gemini/commands/, .agents/skills/, .opencode/commands/
  - Agents:   .claude/agents/   -> .codex/agents/, .gemini/agents/, .agents/agents/, .opencode/agents/
  - Hooks:    .claude/settings.json -> .codex/hooks.json, .agents/hooks.json
  - Path rules: .claude/rules/  -> directory-scoped AGENTS.md approximations, .agents/rules/

Global ($CLAUDE_CONFIG_DIR):
  - commands/  -> files/.agents/skills/, files/.gemini/commands/, files/.config/opencode/commands/
  - agents/    -> files/.codex/agents/, files/.gemini/agents/, files/.gemini/config/agents/, files/.config/opencode/agents/
  - skills/    -> ~/.codex/skills/, ~/.gemini/skills/, ~/.gemini/config/skills/, files/.config/opencode/skills/
  - hooks      -> files/.codex/hooks.json, files/.gemini/settings.json (rewrites ~ in command paths to /home/node for sandbox),
                  files/.gemini/config/hooks.json (Antigravity expands ~ itself)
                  (opencode reuses the hook scripts via files/.config/opencode/plugins/, hand-maintained)
  - CLAUDE.md  -> files/.codex/AGENTS.md, files/.gemini/AGENTS.md, files/.gemini/GEMINI.md (Antigravity reads its global context there), files/.config/opencode/AGENTS.md

Note: the Global files/ outputs above are symlinked into ~ by setup.sh, so they stay
under version control. The project-level AGENTS.md approximations are not, since they
cite repository-relative paths. Global skills are written straight to ~ because codex
owns ~/.codex/skills/.system/ and must not have it captured by the repository.
EOS
}

ENTRYPOINT=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
readonly ENTRYPOINT
ROOT=$(cd "${ENTRYPOINT}/.." && pwd)
readonly ROOT
readonly CLAUDE_CONFIG_DIR="${CLAUDE_CONFIG_DIR:-${HOME}/.config/claudex/config}"
# us-docker.pkg.dev/gemini-code-dev/gemini-cli/sandbox runs as 'node'
# HOME=/home/node
readonly GEMINI_SANDBOX_HOME="/home/node"

while (( $# )); do
  case "$1" in
    -h|--help)
      usage
      exit 0
      ;;
    --)
      shift
      break
      ;;
    -*|--*)
      echo "Unsupported flag $1" 1>&2
      exit 1
      ;;
    *)
      shift
      ;;
  esac
done

convert_skill() {
  local source="$1" destination="$2" content frontmatter name description keywords body short
  content=$(cat "$source")
  if [[ ! "$content" =~ ^--- ]]; then
    echo "[WARN] No frontmatter: $source" 1>&2
    return
  fi

  frontmatter=$(echo "$content" | awk '/^---$/{if(++c==2)exit}c' | tail -n +2)
  name=$(echo "$frontmatter" | awk -F': ' '/^name:/{print $2}')
  description=$(echo "$frontmatter" | awk -F': ' '/^description:/{$1=""; sub(/^ /, ""); print}')
  keywords=$(echo "$frontmatter" | awk -F': ' '/^keywords:/{$1=""; sub(/^ /, ""); print}')
  body=$(echo "$content" | awk '/^---$/{if(++c==2){p=1;next}}p')

  [ "${#description}" -gt 500 ] && description="${description:0:497}..."

  mkdir -p "$(dirname "$destination")"

  if [ -n "$keywords" ]; then
    short=$(echo "$keywords" | xargs -r)
    cat > "$destination" <<EOS
---
# Generated by sync-agent-files.sh. DO NOT EDIT.
name: $name
description: $description
metadata:
  short-description: $short
---
$body
EOS
  else
    cat > "$destination" <<EOS
---
# Generated by sync-agent-files.sh. DO NOT EDIT.
name: $name
description: $description
---
$body
EOS
  fi
  echo "$source -> $destination"
}

convert_command() {
  local source="$1" destination="$2" content frontmatter description body
  content=$(cat "$source")
  if [[ ! "$content" =~ ^--- ]]; then
    echo "[WARN] No frontmatter: $source" 1>&2
    return
  fi

  frontmatter=$(echo "$content" | awk '/^---$/{if(++c==2)exit}c' | tail -n +2)
  description=$(echo "$frontmatter" | awk -F': ' '/^description:/{$1=""; sub(/^ /, ""); print}')
  body=$(echo "$content" \
    | awk '/^---$/{if(++c==2){p=1;next}}p' \
    | sed 's/\$ARGUMENTS/{{args}}/g' \
    | sed 's/^!`\(.*\)`$/!{\1}/')

  mkdir -p "$(dirname "$destination")"
  {
    echo "# Generated by sync-agent-files.sh. DO NOT EDIT."
    printf 'description = %s\n' "$(printf '%s' "$description" | jq -Rs .)"
    printf 'prompt = %s\n' "$(printf '%s\n' "$body" | jq -Rs .)"
  } > "$destination"
  echo "$source -> $destination"
}

convert_command_opencode() {
  local source="$1" destination="$2" content frontmatter description body
  content=$(cat "$source")
  if [[ ! "$content" =~ ^--- ]]; then
    echo "[WARN] No frontmatter: $source" 1>&2
    return
  fi

  frontmatter=$(echo "$content" | awk '/^---$/{if(++c==2)exit}c' | tail -n +2)
  description=$(echo "$frontmatter" | awk -F': ' '/^description:/{$1=""; sub(/^ /, ""); print}')
  body=$(echo "$content" | awk '/^---$/{if(++c==2){p=1;next}}p')

  mkdir -p "$(dirname "$destination")"
  cat > "$destination" <<EOS
---
# Generated by sync-agent-files.sh. DO NOT EDIT.
description: $description
---
$body
EOS
  echo "$source -> $destination"
}

convert_command_skill() {
  local source="$1" destination="$2" name="$3" content frontmatter description argument_hint disable_model_invocation body skill_directory
  content=$(cat "$source")
  if [[ ! "$content" =~ ^--- ]]; then
    echo "[WARN] No frontmatter: $source" 1>&2
    return
  fi

  frontmatter=$(echo "$content" | awk '/^---$/{if(++c==2)exit}c' | tail -n +2)
  description=$(echo "$frontmatter" | awk -F': ' '/^description:/{$1=""; sub(/^ /, ""); print}')
  argument_hint=$(echo "$frontmatter" | awk -F': ' '/^argument-hint:/{$1=""; sub(/^ /, ""); print}')
  disable_model_invocation=$(echo "$frontmatter" | awk -F': ' '/^disable-model-invocation:/{$1=""; sub(/^ /, ""); print}')
  body=$(echo "$content" \
    | awk '/^---$/{if(++c==2){p=1;next}}p' \
    | sed 's/`\$ARGUMENTS`/the arguments provided by the user/g' \
    | sed 's/\$ARGUMENTS/the arguments provided by the user/g' \
    | sed 's/^!`\(.*\)`$/Run this command and use its output as context: `\1`/')

  [ "${#description}" -gt 500 ] && description="${description:0:497}..."

  skill_directory=$(dirname "$destination")
  mkdir -p "$skill_directory"
  {
    echo "---"
    echo "# Generated by sync-agent-files.sh. DO NOT EDIT."
    echo "name: $name"
    echo "description: $description"
    [ -n "$argument_hint" ] && printf 'argument-hint: %s\n' "$(printf '%s' "$argument_hint" | jq -Rs .)"
    [ -n "$disable_model_invocation" ] && echo "disable-model-invocation: $disable_model_invocation"
    echo "---"
    echo "$body"
  } > "$destination"
  echo "$source -> $destination"

  if [ "$disable_model_invocation" = "true" ]; then
    mkdir -p "${skill_directory}/agents"
    cat > "${skill_directory}/agents/openai.yaml" <<'EOS'
# Generated by sync-agent-files.sh. DO NOT EDIT.
policy:
  allow_implicit_invocation: false
EOS
    echo "$source -> ${skill_directory}/agents/openai.yaml"
  else
    rm -f "${skill_directory}/agents/openai.yaml"
  fi
}

convert_agent_opencode() {
  local source="$1" destination="$2" content frontmatter description tools body permission tokens scoped command
  # Some agent files carry a leading HTML comment before the frontmatter.
  content=$(sed -n '/^---$/,$p' "$source")
  if [[ ! "$content" =~ ^--- ]]; then
    echo "[WARN] No frontmatter: $source" 1>&2
    return
  fi

  frontmatter=$(echo "$content" | awk '/^---$/{if(++c==2)exit}c' | tail -n +2)
  description=$(echo "$frontmatter" | awk -F': ' '/^description:/{$1=""; sub(/^ /, ""); print}')
  tools=$(echo "$frontmatter" | awk -F': ' '/^tools:/{$1=""; sub(/^ /, ""); print}')
  body=$(echo "$content" | awk '/^---$/{if(++c==2){p=1;next}}p')

  # Translate Claude's tools allow-list into opencode deny permissions so that restricted agents (e.g. read-only reviewers) keep their guarantees.
  # Match whole tokens: TodoWrite is not Write, Bash(find:*) is not the bare "*"
  # A scoped Bash(cmd:*) grant becomes opencode's per-command bash map
  permission=""
  tokens=",${tools// /},"
  case "$tokens" in
    ",,"|*,\*,*) ;;
    *)
      case "$tokens" in *,Edit,*|*,Write,*|*,MultiEdit,*) ;; *) permission="${permission}"$'  edit: deny\n';; esac
      # Allow leading space after the comma in `Read, Bash(git diff:*)`
      scoped=$(echo "$tools" | tr ',' '\n' | sed -n 's/^ *Bash(\(.*\))$/\1/p')
      case "$tokens" in
        *,Bash,*) ;;
        *)
          if [ -n "$scoped" ]; then
            permission="${permission}"$'  bash:\n    "*": deny\n'
            while IFS= read -r command; do
              case "$command" in
                *:\*)
                  # `Bash(cmd:*)` grants any invocation; opencode matches literally
                  # `cmd *` alone would deny a bare `cmd`
                  command="${command%:\*}"
                  permission="${permission}"'    "'"${command}"'": allow'$'\n'
                  permission="${permission}"'    "'"${command}"' *": allow'$'\n'
                  ;;
                *)
                  # `Bash(cmd)` grants only the bare command, so no ` *` form.
                  permission="${permission}"'    "'"${command}"'": allow'$'\n'
                  ;;
              esac
            done <<< "$scoped"
          else
            permission="${permission}"$'  bash: deny\n'
          fi
          ;;
      esac
      case "$tokens" in *,WebFetch,*|*,WebSearch,*) ;; *) permission="${permission}"$'  webfetch: deny\n';; esac
      ;;
  esac
  [ -n "$permission" ] && permission=$'permission:\n'"${permission}"

  mkdir -p "$(dirname "$destination")"
  cat > "$destination" <<EOS
---
# Generated by sync-agent-files.sh. DO NOT EDIT.
description: $description
mode: subagent
${permission}---
$body
EOS
  echo "$source -> $destination"
}

convert_agent_codex() {
  local source="$1" destination="$2" content frontmatter name description tools body
  content=$(sed -n '/^---$/,$p' "$source")
  if [[ ! "$content" =~ ^--- ]]; then
    echo "[WARN] No frontmatter: $source" 1>&2
    return
  fi

  frontmatter=$(echo "$content" | awk '/^---$/{if(++c==2)exit}c' | tail -n +2)
  name=$(echo "$frontmatter" | awk -F': ' '/^name:/{print $2}')
  description=$(echo "$frontmatter" | awk -F': ' '/^description:/{$1=""; sub(/^ /, ""); print}')
  tools=$(echo "$frontmatter" | awk -F': ' '/^tools:/{$1=""; sub(/^ /, ""); print}')
  body=$(echo "$content" | awk '/^---$/{if(++c==2){p=1;next}}p')
  [ -n "$name" ] || name=$(basename "$destination" .toml)

  mkdir -p "$(dirname "$destination")"
  {
    echo "# Generated by sync-agent-files.sh. DO NOT EDIT."
    printf 'name = %s\n' "$(printf '%s' "$name" | jq -Rs .)"
    printf 'description = %s\n' "$(printf '%s' "$description" | jq -Rs .)"
    case ",${tools// /}," in
      ,,|*,Edit,*|*,Write,*|*,MultiEdit,*|*,Bash\(rm:\*\),*|*,\*,*) ;;
      *) echo 'sandbox_mode = "read-only"' ;;
    esac
    printf 'developer_instructions = %s\n' "$(printf '%s\n' "$body" | jq -Rs .)"
  } > "$destination"
  echo "$source -> $destination"
}

convert_agent_gemini() {
  local source="$1" destination="$2" content frontmatter description tools body name gemini_tools tool gemini_tool tools_yaml
  # Some agent files carry a leading HTML comment before the frontmatter.
  content=$(sed -n '/^---$/,$p' "$source")
  if [[ ! "$content" =~ ^--- ]]; then
    echo "[WARN] No frontmatter: $source" 1>&2
    return
  fi

  frontmatter=$(echo "$content" | awk '/^---$/{if(++c==2)exit}c' | tail -n +2)
  description=$(echo "$frontmatter" | awk -F': ' '/^description:/{$1=""; sub(/^ /, ""); print}')
  tools=$(echo "$frontmatter" | awk -F': ' '/^tools:/{$1=""; sub(/^ /, ""); print}')
  body=$(echo "$content" | awk '/^---$/{if(++c==2){p=1;next}}p')
  # Derive the subagent name from the file name.
  # Gemini requires the slug to use only lowercase letters, numbers, hyphens, and underscores, so sanitize the colon used to flatten nested global agent paths.
  name=$(basename "$destination" .md | tr '[:upper:]' '[:lower:]')
  name="${name//[^a-z0-9_-]/-}"

  gemini_tools=""
  while IFS= read -r tool; do
    case "$tool" in
      Bash) gemini_tool="run_shell_command" ;;
      Bash\(*\))
        echo "[WARN] Gemini cannot enforce a scoped shell allow-list in an agent file: $tool in $source" 1>&2
        continue
        ;;
      Edit|MultiEdit) gemini_tool="replace" ;;
      Glob) gemini_tool="glob" ;;
      Grep) gemini_tool="grep_search" ;;
      LS) gemini_tool="list_directory" ;;
      Read) gemini_tool="read_file" ;;
      Skill) gemini_tool="activate_skill" ;;
      TodoRead|TodoWrite) gemini_tool="write_todos" ;;
      WebFetch) gemini_tool="web_fetch" ;;
      WebSearch) gemini_tool="google_web_search" ;;
      Write) gemini_tool="write_file" ;;
      \*|"") gemini_tool="*" ;;
      Task) continue ;;
      mcp__*)
        gemini_tool="${tool/mcp__/mcp_}"
        gemini_tool="${gemini_tool//__/_}"
        ;;
      *) echo "[WARN] Unknown Gemini tool mapping: $tool in $source" 1>&2; continue ;;
    esac
    case "
${gemini_tools}
" in
      *"
${gemini_tool}
"*) ;;
      *) gemini_tools="${gemini_tools}${gemini_tool}"$'\n' ;;
    esac
  done < <(echo "$tools" | tr ',' '\n' | sed 's/^ *//; s/ *$//')

  tools_yaml=""
  if [ -n "$tools" ]; then
    if [ -n "$gemini_tools" ]; then
      tools_yaml="tools:"
      while IFS= read -r gemini_tool; do
        [ -n "$gemini_tool" ] && tools_yaml="${tools_yaml}"$'\n'"  - ${gemini_tool}"
      done <<< "$gemini_tools"
    else
      tools_yaml="tools: []"
    fi
  fi

  mkdir -p "$(dirname "$destination")"
  cat > "$destination" <<EOS
---
# Generated by sync-agent-files.sh. DO NOT EDIT.
name: $name
description: $description
kind: local
${tools_yaml}
---
$body
EOS
  echo "$source -> $destination"
}

convert_agent_antigravity() {
  local source="$1" destination="$2" content frontmatter description tools body name antigravity_tools tool antigravity_tool tools_yaml
  # Some agent files carry a leading HTML comment before the frontmatter.
  content=$(sed -n '/^---$/,$p' "$source")
  if [[ ! "$content" =~ ^--- ]]; then
    echo "[WARN] No frontmatter: $source" 1>&2
    return
  fi

  frontmatter=$(echo "$content" | awk '/^---$/{if(++c==2)exit}c' | tail -n +2)
  description=$(echo "$frontmatter" | awk -F': ' '/^description:/{$1=""; sub(/^ /, ""); print}')
  tools=$(echo "$frontmatter" | awk -F': ' '/^tools:/{$1=""; sub(/^ /, ""); print}')
  body=$(echo "$content" | awk '/^---$/{if(++c==2){p=1;next}}p')
  name=$(basename "$destination" .md | tr '[:upper:]' '[:lower:]')
  name="${name//[^a-z0-9_-]/-}"

  antigravity_tools=""
  add_antigravity_tool() {
    case "
${antigravity_tools}
" in
      *"
$1
"*) ;;
      *) antigravity_tools="${antigravity_tools}$1"$'\n' ;;
    esac
  }

  if [ -z "$tools" ]; then
    for antigravity_tool in read_file edit_file write_file run_command manage_task search_web; do
      add_antigravity_tool "$antigravity_tool"
    done
  else
    while IFS= read -r tool; do
      case "$tool" in
        Bash)
          add_antigravity_tool "run_command"
          add_antigravity_tool "manage_task"
          ;;
        'Bash(rm:*)') add_antigravity_tool "run_command" ;;
        Bash\(*\))
          echo "[WARN] Antigravity cannot enforce a scoped shell allow-list in an agent file: $tool in $source" 1>&2
          ;;
        Edit|MultiEdit) add_antigravity_tool "edit_file" ;;
        Write) add_antigravity_tool "write_file" ;;
        Read|Glob|Grep|LS) add_antigravity_tool "read_file" ;;
        Task) add_antigravity_tool "invoke_subagent" ;;
        WebFetch|WebSearch) add_antigravity_tool "search_web" ;;
        \*)
          for antigravity_tool in read_file edit_file write_file run_command manage_task search_web; do
            add_antigravity_tool "$antigravity_tool"
          done
          ;;
        ""|Skill|TodoRead|TodoWrite|NotebookEdit|mcp__*) ;;
        *) echo "[WARN] Unknown Antigravity tool mapping: $tool in $source" 1>&2 ;;
      esac
    done < <(echo "$tools" | tr ',' '\n' | sed 's/^ *//; s/ *$//')
  fi

  if [ -n "$antigravity_tools" ]; then
    tools_yaml="tools:"
    while IFS= read -r antigravity_tool; do
      [ -n "$antigravity_tool" ] && tools_yaml="${tools_yaml}"$'\n'"  - ${antigravity_tool}"
    done <<< "$antigravity_tools"
  else
    tools_yaml="tools: []"
  fi

  mkdir -p "$(dirname "$destination")"
  cat > "$destination" <<EOS
---
# Generated by sync-agent-files.sh. DO NOT EDIT.
name: $name
description: $description
subagent: true
mainAgent: false
${tools_yaml}
---
$body
EOS
  echo "$source -> $destination"
}

convert_hooks_codex() {
  local source="$1" destination="$2" hooks

  hooks=$(jq '
    {
      description: "Generated from Claude Code hooks by sync-agent-files.sh.",
      hooks: (
        (.hooks // {})
        | with_entries(select(.key == "PreToolUse"
          or .key == "PostToolUse"
          or .key == "UserPromptSubmit"
          or .key == "Stop"
          or .key == "SubagentStop"
          or .key == "SessionStart"
          or .key == "SessionEnd"
          or .key == "PreCompact"))
        | if .PostToolUse then
            .PostToolUse |= map(
              if (.matcher? | type) == "string" and (.matcher | test("Write|Edit|MultiEdit")) then
                .matcher |= (split("|") + ["apply_patch"] | unique | join("|"))
              else .
              end
            )
          else .
          end
      )
    }
  ' "$source")

  [ "$(echo "$hooks" | jq '.hooks | length')" -gt 0 ] || return 0
  mkdir -p "$(dirname "$destination")"
  echo "$hooks" > "$destination"
  echo "hooks -> $destination"
}

convert_hooks_gemini() {
  local source="$1" destination="$2" hooks mapped

  hooks=$(jq -e '.hooks // empty' "$source" 2>/dev/null) || hooks=""
  [ -n "$hooks" ] || return 0

  mapped=$(echo "$hooks" | jq --arg home "$GEMINI_SANDBOX_HOME" '
    to_entries | [.[] |
      select((.key == "SubagentStop") | not) |
      if .key == "PreToolUse" then .key = "BeforeTool"
      elif .key == "PostToolUse" then .key = "AfterTool"
      elif .key == "UserPromptSubmit" then .key = "BeforeAgent"
      elif .key == "Stop" then .key = "AfterAgent"
      else .
      end
    ] | from_entries
    | walk(
        if type == "object" and (.matcher? | type) == "string" and .matcher != "" then
          .matcher |= (
            split("|")
            | map(
                if . == "Bash" then ["run_shell_command"]
                elif . == "Write" or . == "Edit" or . == "MultiEdit" or . == "NotebookEdit" then ["write_file", "replace"]
                else [.]
                end
              )
            | flatten | unique | join("|")
          )
        else .
        end
      )
    | walk(
        if type == "object" and (.timeout? | type) == "number" then
          .timeout *= 1000
        else .
        end
      )
    | walk(if type == "object" and .type == "command" and (.command? | type) == "string" then .command |= sub("^~/"; $home + "/") else . end)
  ')

  mkdir -p "$(dirname "$destination")"
  if [ -f "$destination" ]; then
    if ! jq --argjson hooks "$mapped" '.hooks = $hooks' "$destination" > "${destination}.tmp"; then
      rm -f "${destination}.tmp"
      exit 1
    fi
    mv "${destination}.tmp" "$destination"
  else
    echo "{\"hooks\": {}}" | jq --argjson hooks "$mapped" '.hooks += $hooks' > "$destination"
  fi
  echo "hooks -> $destination"
}

convert_hooks_antigravity() {
  local source="$1" destination="$2" name="${3:-claudex}" hooks mapped

  hooks=$(jq -e '.hooks // empty' "$source" 2>/dev/null) || hooks=""
  [ -n "$hooks" ] || return 0

  # https://antigravity.google/docs/hooks#supported-events
  mapped=$(echo "$hooks" | jq --arg name "$name" '
    to_entries | [.[] |
      select(.key == "PreToolUse" or .key == "PostToolUse" or .key == "UserPromptSubmit" or .key == "SessionStart" or .key == "Stop") |
      if .key == "UserPromptSubmit" then {key: "PreInvocation", value: [.value[].hooks[]]}
      elif .key == "SessionStart" or .key == "Stop" then .value |= [.[] | .hooks[]]
      else .
      end
    ] | from_entries
    | walk(
        if type == "object" and (.matcher? | type) == "string" and .matcher != "" then
          .matcher |= (
            split("|")
            | map(
                if . == "Bash" then ["run_command"]
                elif . == "Write" then ["write_file", "write_to_file"]
                elif . == "Edit" then ["edit_file", "replace_file_content"]
                elif . == "MultiEdit" then ["edit_file", "multi_replace_file_content"]
                elif . == "ExitPlanMode" or . == "NotebookEdit" then []
                else [.]
                end
              )
            | flatten | unique | join("|")
          )
        else .
        end
      )
    | {($name): .}
  ')

  [ "$(echo "$mapped" | jq 'first(.[]) | length')" -gt 0 ] || return 0
  mkdir -p "$(dirname "$destination")"
  echo "$mapped" > "$destination"
  echo "hooks -> $destination"
}

generated_instructions_file() {
  sed -n '1{/^---$/!{p;q}}; 1,/^---$/d; p;q' "$1" | grep -qx '<!-- Generated by sync-agent-files.sh. DO NOT EDIT. -->'
}

convert_path_scoped_instructions() {
  local rules_directory="$1" rule relative frontmatter body path static_prefix destination
  local destination_paths rendered_paths exact_directory_scope generated_file
  local antigravity_destination antigravity_paths
  local -A initialized_destinations=()
  local -A destinations_for_rule=()
  local -A exact_paths_for_rule=()
  local -A conditional_paths_for_rule=()
  local -A antigravity_destinations=()

  while IFS= read -r generated_file; do
    if generated_instructions_file "$generated_file" \
      && grep -Eq '^## (Conditional rule: )?\.claude/rules/' "$generated_file"; then
      rm "$generated_file"
    fi
  done < <(find "$ROOT" -name AGENTS.md -type f)

  while IFS= read -r rule; do
    relative="${rule#"${rules_directory}/"}"
    frontmatter=$(sed -n '1{/^---$/!q}; 2,/^---$/p' "$rule" | sed '$d')
    body=$(awk '/^---$/{if(++count==2){print_after=1; next}} print_after' "$rule")
    destinations_for_rule=()
    exact_paths_for_rule=()
    conditional_paths_for_rule=()
    antigravity_paths=()

    while IFS= read -r path; do
      [ -n "$path" ] || continue
      path="${path#\"}"
      path="${path%\"}"
      case "$path" in
        /*|..|../*|*/../*)
          echo "[WARN] Path matcher escapes the project; keeping it at the project root: $path in $rule" 1>&2
          destination="${ROOT}/AGENTS.md"
          ;;
        *)
          antigravity_paths+=("$path")
          static_prefix="${path%%[*?\{]*}"
          static_prefix="${static_prefix%/}"
          if [ -z "$static_prefix" ]; then
            destination="${ROOT}/AGENTS.md"
          elif [ -d "${ROOT}/${static_prefix}" ]; then
            destination="${ROOT}/${static_prefix}/AGENTS.md"
          else
            destination="${ROOT}/$(dirname "$static_prefix")/AGENTS.md"
          fi
          ;;
      esac
      destination=$(realpath -m "$destination")

      destinations_for_rule["$destination"]=true

      exact_directory_scope=false
      case "$path" in
        *"/**")
          case "${path%/\*\*}" in
            *[\*\?\{\[]*) ;;
            *) exact_directory_scope=true ;;
          esac
          ;;
      esac
      if [ "$exact_directory_scope" = true ]; then
        destination_paths="${exact_paths_for_rule["$destination"]-}"
        exact_paths_for_rule["$destination"]="${destination_paths}${path}"$'\n'
      else
        destination_paths="${conditional_paths_for_rule["$destination"]-}"
        conditional_paths_for_rule["$destination"]="${destination_paths}${path}"$'\n'
        echo "[WARN] AGENTS.md cannot enforce path matcher '${path}'; generating a conditional source reference at $destination" 1>&2
      fi
    done < <(echo "$frontmatter" | sed -n 's/^  - *//p')

    if [ "${#antigravity_paths[@]}" -gt 0 ] || [ "${#destinations_for_rule[@]}" -eq 0 ]; then
      antigravity_destination="${ROOT}/.agents/rules/claude-${relative//\//--}"
      antigravity_destinations["$antigravity_destination"]=true
      if [ ! -f "$antigravity_destination" ] || generated_instructions_file "$antigravity_destination"; then
        mkdir -p "$(dirname "$antigravity_destination")"
        {
          echo "---"
          if [ "${#antigravity_paths[@]}" -gt 0 ]; then
            rendered_paths=$(IFS=,; printf '%s' "${antigravity_paths[*]}" | jq -Rs .)
            echo "trigger: glob"
            echo "globs: $rendered_paths"
          else
            echo "trigger: always_on"
          fi
          echo "---"
          echo "<!-- Generated by sync-agent-files.sh. DO NOT EDIT. -->"
          echo "$body"
        } > "$antigravity_destination"
        echo "$rule -> $antigravity_destination"
      fi
    fi

    if [ "${#destinations_for_rule[@]}" -eq 0 ]; then
      destinations_for_rule["${ROOT}/AGENTS.md"]=true
      exact_paths_for_rule["${ROOT}/AGENTS.md"]=""
    fi

    for destination in "${!destinations_for_rule[@]}"; do
      if [ -z "${initialized_destinations["$destination"]+set}" ]; then
        if [ -f "$destination" ] && ! generated_instructions_file "$destination"; then
          echo "[WARN] Existing AGENTS.md is not generated; skipping path-scoped instructions for $destination" 1>&2
          initialized_destinations["$destination"]="skip"
        else
          mkdir -p "$(dirname "$destination")"
          # Claude Code loads every *.md under a commands root as a slash command
          case "$destination" in
            */.claude/commands/*|*/claudex/config/commands/*)
              printf -- '---\ndescription: Not a command - directory-scoped instructions for agents that read AGENTS.md\ndisable-model-invocation: true\n---\n' > "$destination"
              echo "<!-- Generated by sync-agent-files.sh. DO NOT EDIT. -->" >> "$destination"
              ;;
            *)
              echo "<!-- Generated by sync-agent-files.sh. DO NOT EDIT. -->" > "$destination"
              ;;
          esac
          initialized_destinations["$destination"]="write"
        fi
      fi
      [ "${initialized_destinations["$destination"]}" = "write" ] || continue

      if [ -n "${exact_paths_for_rule["$destination"]+set}" ]; then
        rendered_paths=$(printf '%s' "${exact_paths_for_rule["$destination"]}" | sed '/^$/d; s/^/`/; s/$/`/' | paste -sd, - | sed 's/,/, /g')
        {
          echo
          echo "## .claude/rules/${relative}"
          [ -n "$rendered_paths" ] && echo
          [ -n "$rendered_paths" ] && echo "Directory scope: ${rendered_paths}"
          echo
          echo "$body"
        } >> "$destination"
      fi
      if [ -n "${conditional_paths_for_rule["$destination"]+set}" ]; then
        rendered_paths=$(printf '%s' "${conditional_paths_for_rule["$destination"]}" | sed '/^$/d; s/^/`/; s/$/`/' | paste -sd, - | sed 's/,/, /g')
        {
          echo
          echo "## Conditional rule: .claude/rules/${relative}"
          echo
          echo "- When a target path matches ${rendered_paths}, read and follow \`.claude/rules/${relative}\` before proceeding."
        } >> "$destination"
      fi
      echo "$rule -> $destination"
    done
  done < <(find -L "$rules_directory" -name "*.md" ! -name "AGENTS.md" -type f | LC_ALL=C sort)

  for generated_file in "${ROOT}"/.agents/rules/claude-*.md; do
    if [ -f "$generated_file" ] && [ -z "${antigravity_destinations["$generated_file"]+set}" ] && generated_instructions_file "$generated_file"; then
      rm "$generated_file"
    fi
  done
}

configure_codex_project_instructions() {
  local destination="${ROOT}/.codex/config.toml"

  if [ -f "$destination" ]; then
    if ! grep -q '^project_doc_max_bytes *= *' "$destination"; then
      echo "[WARN] Set project_doc_max_bytes to at least 262144 in $destination" 1>&2
    fi
    return 0
  fi

  mkdir -p "$(dirname "$destination")"
  cat > "$destination" <<EOS
# Generated by sync-agent-files.sh. DO NOT EDIT.
project_doc_max_bytes = 262144
EOS
  echo "project instructions configuration -> $destination"
}

# Project skills
if [ -d "${ROOT}/.claude/skills" ]; then
  while IFS= read -r f; do
    name=$(dirname "$f" | xargs -r basename)
    convert_skill "$f" "${ROOT}/.gemini/skills/${name}/SKILL.md"
    convert_skill "$f" "${ROOT}/.agents/skills/${name}/SKILL.md"
  done < <(find -L "${ROOT}/.claude/skills" -name "SKILL.md" -type f)

  while IFS= read -r f; do
    relative="${f#"${ROOT}/.claude/skills/"}"
    mkdir -p "$(dirname "${ROOT}/.gemini/skills/${relative}")"
    cp "$f" "${ROOT}/.gemini/skills/${relative}"
    mkdir -p "$(dirname "${ROOT}/.agents/skills/${relative}")"
    cp "$f" "${ROOT}/.agents/skills/${relative}"
  done < <(find -L "${ROOT}/.claude/skills" -type f ! -name "SKILL.md" ! -name "AGENTS.md")
fi

# Project commands
if [ -d "${ROOT}/.claude/commands" ]; then
  while IFS= read -r f; do
    relative="${f#"${ROOT}/.claude/commands/"}"
    name="${relative%.md}"
    command_name="${name//\//:}"
    skill_name="${name//\//-}"
    convert_command "$f" "${ROOT}/.gemini/commands/${command_name}.toml"
    convert_command_skill "$f" "${ROOT}/.agents/skills/${skill_name}/SKILL.md" "$skill_name"
    convert_command_opencode "$f" "${ROOT}/.opencode/commands/${command_name}.md"
  done < <(find -L "${ROOT}/.claude/commands" -name "*.md" ! -name "AGENTS.md" -type f)
fi

# Project agents
if [ -d "${ROOT}/.claude/agents" ]; then
  while IFS= read -r f; do
    name=$(basename "$f" .md)
    convert_agent_codex "$f" "${ROOT}/.codex/agents/${name}.toml"
    convert_agent_opencode "$f" "${ROOT}/.opencode/agents/${name}.md"
    convert_agent_gemini "$f" "${ROOT}/.gemini/agents/${name}.md"
    convert_agent_antigravity "$f" "${ROOT}/.agents/agents/${name}.md"
  done < <(find -L "${ROOT}/.claude/agents" -name "*.md" ! -name "AGENTS.md" -type f)
fi

# Project hooks
if [ -f "${ROOT}/.claude/settings.json" ]; then
  for client in codex antigravity; do
    case "$client" in
      codex)
        destination="${ROOT}/.codex/hooks.json"
        user_hooks='${CODEX_HOME:-$HOME/.codex}/hooks.json'
        ;;
      antigravity)
        destination="${ROOT}/.agents/hooks.json"
        user_hooks='$HOME/.gemini/config/hooks.json'
        ;;
    esac
    project_settings=$(jq --arg user_hooks "$user_hooks" '
      (.hooks[][] .hooks[] | select(.type == "command") | .command) |= (
        if test("^[$]CLAUDE_PROJECT_DIR/\\.claude/hooks/(UserPromptSubmit|PreToolUse|PostToolUse|Stop)\\.sh$") then
          (split("/") | last) as $script
          | ("~/.config/claudex/config/hooks/" + $script) as $global_command
          | (".. | .command? // empty | select(. == $command)" | @sh) as $filter
          | "if ! jq -e --arg command \($global_command | @sh) \($filter) \"\($user_hooks)\" >/dev/null 2>&1; then exec \"$(git rev-parse --show-toplevel)/files/home/kai/.config/claudex/config/hooks/\($script)\"; fi"
        else
          gsub("[$]CLAUDE_PROJECT_DIR"; "\"$(git rev-parse --show-toplevel)\"")
        end
      )
    ' "${ROOT}/.claude/settings.json")
    "convert_hooks_${client}" <(printf '%s\n' "$project_settings") "$destination" claudex-project
  done
fi

# Project path-scoped instructions
if [ -d "${ROOT}/.claude/rules" ]; then
  convert_path_scoped_instructions "${ROOT}/.claude/rules"
  configure_codex_project_instructions
fi

# Global configs
[ -d "$CLAUDE_CONFIG_DIR" ] || exit 0

# Global commands
if [ -d "${CLAUDE_CONFIG_DIR}/commands" ]; then
  while IFS= read -r f; do
    relative="${f#"${CLAUDE_CONFIG_DIR}/commands/"}"
    name="${relative%.md}"
    command_name="${name//\//:}"
    skill_name="${name//\//-}"
    convert_command_skill "$f" "${ROOT}/files/home/kai/.agents/skills/${skill_name}/SKILL.md" "$skill_name"
    convert_command "$f" "${ROOT}/files/home/kai/.gemini/commands/${command_name}.toml"
    convert_command_opencode "$f" "${ROOT}/files/home/kai/.config/opencode/commands/${command_name}.md"
  done < <(find -L "${CLAUDE_CONFIG_DIR}/commands" -name "*.md" ! -name "AGENTS.md" -type f)
fi

# Global agents
if [ -d "${CLAUDE_CONFIG_DIR}/agents" ]; then
  while IFS= read -r f; do
    relative="${f#"${CLAUDE_CONFIG_DIR}/agents/"}"
    name="${relative%.md}"
    name="${name//\//:}"
    convert_agent_codex "$f" "${ROOT}/files/home/kai/.codex/agents/${name}.toml"
    convert_agent_opencode "$f" "${ROOT}/files/home/kai/.config/opencode/agents/${name}.md"
    convert_agent_gemini "$f" "${ROOT}/files/home/kai/.gemini/agents/${name}.md"
    convert_agent_antigravity "$f" "${ROOT}/files/home/kai/.gemini/config/agents/${name}.md"
  done < <(find -L "${CLAUDE_CONFIG_DIR}/agents" -name "*.md" ! -name "AGENTS.md" -type f)
fi

# Global skills
if [ -d "${CLAUDE_CONFIG_DIR}/skills" ]; then
  while IFS= read -r f; do
    name=$(dirname "$f" | xargs -r basename)
    convert_skill "$f" ~/.codex/skills/"${name}"/SKILL.md
    convert_skill "$f" ~/.gemini/skills/"${name}"/SKILL.md
    convert_skill "$f" ~/.gemini/config/skills/"${name}"/SKILL.md
    convert_skill "$f" "${ROOT}/files/home/kai/.config/opencode/skills/${name}/SKILL.md"
  done < <(find -L "${CLAUDE_CONFIG_DIR}/skills" -name "SKILL.md" -type f)

  while IFS= read -r f; do
    relative="${f#"${CLAUDE_CONFIG_DIR}/skills/"}"
    mkdir -p "$(dirname ~/.codex/skills/"${relative}")"
    cp "$f" ~/.codex/skills/"${relative}"
    mkdir -p "$(dirname ~/.gemini/skills/"${relative}")"
    cp "$f" ~/.gemini/skills/"${relative}"
    mkdir -p "$(dirname ~/.gemini/config/skills/"${relative}")"
    cp "$f" ~/.gemini/config/skills/"${relative}"
    mkdir -p "$(dirname "${ROOT}/files/home/kai/.config/opencode/skills/${relative}")"
    cp "$f" "${ROOT}/files/home/kai/.config/opencode/skills/${relative}"
  done < <(find -L "${CLAUDE_CONFIG_DIR}/skills" -type f ! -name "SKILL.md" ! -name "AGENTS.md")
fi

# Global hooks
if [ -f "${CLAUDE_CONFIG_DIR}/settings.json" ]; then
  convert_hooks_codex "${CLAUDE_CONFIG_DIR}/settings.json" "${ROOT}/files/home/kai/.codex/hooks.json"
  convert_hooks_gemini "${CLAUDE_CONFIG_DIR}/settings.json" "${ROOT}/files/home/kai/.gemini/settings.json"
  convert_hooks_antigravity "${CLAUDE_CONFIG_DIR}/settings.json" "${ROOT}/files/home/kai/.gemini/config/hooks.json"
fi

# Global user prompt
if [ -f "${CLAUDE_CONFIG_DIR}/CLAUDE.md" ]; then
  expanded=""
  while IFS= read -r line; do
    if [[ "$line" =~ ^@(.+)$ ]]; then
      f="${CLAUDE_CONFIG_DIR}/${BASH_REMATCH[1]}"
      [ -f "$f" ] && expanded="${expanded}$(cat "$f")"$'\n'
    else
      expanded="${expanded}${line}"$'\n'
    fi
  done < "${CLAUDE_CONFIG_DIR}/CLAUDE.md"

  # Common removals
  base=$(echo "$expanded" \
    | sed '/@[a-z-]*\s*エージェント/d' \
    | sed '/並列:/d' \
    | sed '/backgroundで実行/d' \
    | sed '/`codex` MCP/d' \
    | sed '/`agy` MCP/d' \
    | sed '/OpenAI社からブロック/d' \
    | sed '/`model: "fable"`/d' \
    | sed '/`Plan mode`/d' \
    | cat -s)

  # Git restriction text
  git_restriction=$'\n\n# Git Usage Rules\n- You may use read-only git commands (e.g., `git status`, `git log`, `git diff`) to inspect the repository state.\n- You are STRICTLY FORBIDDEN from using git commands that modify the repository (e.g., `git add`, `git commit`, `git push`, `git checkout`, `git reset`).\n- Do NOT modify the contents of the `.git` directory directly.'

  # Codex: remove TodoWrite/AskUserQuestion/CronCreate, replace TaskCreate/WebSearch
  codex=$(echo "$base" \
    | sed '/TodoWrite/d' \
    | sed '/AskUserQuestion/d' \
    | sed '/CronCreate/d' \
    | sed 's/TaskCreate/update_plan/g' \
    | sed 's/WebSearch/web_search/g')
  codex="${codex}${git_restriction}"
  destination="${ROOT}/files/home/kai/.codex/AGENTS.md"
  mkdir -p "$(dirname "$destination")"
  { echo "<!-- Generated by sync-agent-files.sh. DO NOT EDIT. -->"; echo "$codex"; } > "$destination"
  echo "CLAUDE.md -> $destination"

  # Gemini: replace TodoWrite with write_todos, WebSearch with google_web_search
  gemini=$(echo "$base" \
    | sed 's/TodoWrite/write_todos/g' \
    | sed 's/WebSearch/google_web_search/g')
  gemini="${gemini}${git_restriction}"
  destination="${ROOT}/files/home/kai/.gemini/AGENTS.md"
  mkdir -p "$(dirname "$destination")"
  { echo "<!-- Generated by sync-agent-files.sh. DO NOT EDIT. -->"; echo "$gemini"; } > "$destination"
  echo "CLAUDE.md -> $destination"

  # Antigravity: remove TodoWrite, map Claude Code tool names to Antigravity native tools
  antigravity=$(echo "$base" \
    | sed '/TodoWrite/d' \
    | sed 's/TaskCreate/update_steps/g' \
    | sed 's/CronCreate/schedule/g' \
    | sed 's/CronDelete/manage_task/g' \
    | sed 's/AskUserQuestion/ask_question/g' \
    | sed 's/WebSearch/search_web/g')
  antigravity="${antigravity}${git_restriction}"
  destination="${ROOT}/files/home/kai/.gemini/GEMINI.md"
  mkdir -p "$(dirname "$destination")"
  { echo "<!-- Generated by sync-agent-files.sh. DO NOT EDIT. -->"; echo "$antigravity"; } > "$destination"
  echo "CLAUDE.md -> $destination"

  # opencode: replace TodoWrite/WebSearch, remove AskUserQuestion
  opencode=$(echo "$base" \
    | sed '/AskUserQuestion/d' \
    | sed 's/TodoWrite/todowrite/g' \
    | sed 's/WebSearch/websearch/g')
  opencode="${opencode}${git_restriction}"
  destination="${ROOT}/files/home/kai/.config/opencode/AGENTS.md"
  mkdir -p "$(dirname "$destination")"
  { echo "<!-- Generated by sync-agent-files.sh. DO NOT EDIT. -->"; echo "$opencode"; } > "$destination"
  echo "CLAUDE.md -> $destination"
fi
