---
paths:
  - "files/home/kai/.config/claudex/hooks/*.sh"
---

* Investigate existing hooks to extract conventions before creating new ones
* After creating a hook, register it in `files/home/kai/.config/claudex/settings.json` under `hooks.{HookType}`
* Read JSON input from stdin with `json=$(cat -)`

## Hook Types

| Hook | Trigger | Exit Codes |
|------|---------|------------|
| PreToolUse | Before tool execution | 0=allow, 2=block |
| PostToolUse | After tool execution | 0=success (no action needed) |
| Stop | When Claude stops | 0=allow, JSON with `decision:block`=retry |
| UserPromptSubmit | After user submits prompt | 0=success, stdout appended to context |
| statusLine | Status bar render | stdout=display content |

## Exit Code Semantics

| Exit Code | Meaning |
|-----------|---------|
| 0 | Allow/success |
| 2 | Block execution (PreToolUse only) |

## Blocking Pattern (PreToolUse)

Substring matching (order-independent, handles quoted arguments):

```bash
tool_name=$(echo "$json" | jq -r '.tool_name')

[ "$tool_name" = "Bash" ] || exit 0

command=$(echo "$json" | jq -r '.tool_input.command // empty')

blocked_commands=(
  "gh issue create"
  "rm /"
)

for keyword_set in "${blocked_commands[@]}"; do
  match=true
  for keyword in $keyword_set; do
    if [[ "$command" != *"$keyword"* ]]; then
      match=false
      break
    fi
  done
  if $match; then
    echo "Blocked: contains '$keyword_set'" >&2
    exit 2
  fi
done

exit 0
```

## Stop Compliance Check Pattern

Uses `stop_hook_active` to block once per stop cycle:

```bash
stop_hook_active=$(echo "$json" | jq -r '.stop_hook_active')

if [ "$stop_hook_active" = "true" ]; then
  echo '{}'  # Allow (re-entry after block)
else
  echo '{"decision":"block","reason":"Verify compliance..."}'
fi
```

| Scenario | stop_hook_active | Action |
|----------|------------------|--------|
| First stop attempt | `false` | Block (compliance check) |
| Re-entry after block | `true` | Allow stop |

Do NOT use flag files for coordination between UserPromptSubmit and Stop hooks — UserPromptSubmit may fire on internal events, causing infinite block loops.

## Reference

If blocking specific commands:
  Read: `files/home/kai/.config/claudex/hooks/PreToolUse.sh`

If implementing stop compliance check:
  Read: `files/home/kai/.config/claudex/hooks/Stop.sh`

If appending context to prompts:
  Read: `files/home/kai/.config/claudex/hooks/UserPromptSubmit.sh`
