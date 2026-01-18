---
paths:
  - "**/*.sh"
---

* Use `#!/usr/bin/env bash` as shebang (`#!/usr/bin/env -S bash -l` for login shell)
* Always include `set -eo pipefail` at the top (pipefail catches errors in pipes like `curl | jq`)
* Use `[ ]` for conditionals (`[[ ]]` only when `<`, `>`, or `=~` is required)
* Use `-f` for files and `-d` for directories instead of `-e` (reserve `-e` for sockets, FIFOs, or type-agnostic checks)
* Quote variables with `"$var"` (use `"${var}"` when followed by any character like `"${var}/path"` or `"${var}_suffix"`)
* Use `""` for strings by default, `''` only when shell expansion must be prevented (jq filters, regex patterns)
* For nested quotes (fzf --preview, etc.), use `""` for outer and `''` for inner jq filters
* Use `while IFS= read -r` for file path processing (see subshell note below)
* Use `${XDG_RUNTIME_DIR:-/tmp}` for user-specific runtime files (credentials, session state, keys)
* Use `~` for home directory references (not `$HOME`)

## Loop patterns

| Pattern | Variables | Loop errors | Command errors | Use when |
|---------|-----------|-------------|----------------|----------|
| `while read < <(cmd)` | Preserved | Yes | No | `find`, reliable commands |
| `var=$(cmd); for x in $var` | N/A | Yes | Yes (`pipefail`) | API calls, may-fail commands |
| `cmd \| while read` | Lost | No | Yes (`pipefail`) | Infinite streams (`tail -f`, `kubectl logs -f`) |

### Process substitution (for `find` etc.)

```bash
pids=()
while IFS= read -r file; do
  process "$file" &
  pids+=($!)
done < <(find . -type f -name "*.ext")
```

### Variable + for loop (for API calls)

When the command may fail (network, auth), use variable assignment to detect errors via `pipefail`:

```bash
branches=$(curl -fsSL ... | jq -re '.[].name')

for branch in ${branches}; do
  ...
done
```

### Pipe (for infinite streams)

For commands that never terminate, use pipe to detect abnormal exit via `pipefail`:

```bash
pactl subscribe | while IFS= read -r event; do
  ...
done
```

## xargs

Always use `-r` (--no-run-if-empty) to prevent command execution when input is empty:

| Condition | Pattern |
|-----------|---------|
| Multiple arguments supported | `xargs -r` |
| One-at-a-time processing | `xargs -r -L1` |
| Placeholder substitution | `xargs -r -I{}` |

`-L1` is required when:
- Command processes one argument at a time (`tail -f`, `gh pr merge`)
- Each line needs separate output (`echo`)

`-L1` is NOT required when:
- Command handles multiple arguments (`rm`, `sed`, `dirname`, `chmod`)

Note: `-I{}` implies `-L1` behavior, so explicit `-L1` is optional with `-I{}`.

## grep + pipefail

`grep` returns exit code 1 when no match found, which fails the pipeline with `pipefail`. Use `awk` instead:

| grep | awk equivalent |
|------|----------------|
| `grep pattern` | `awk '/pattern/'` |
| `grep -v pattern` | `awk '!/pattern/'` |
| `grep -E 'a\|b'` | `awk '/a\|b/'` |

## jq + pipefail

`jq -r` outputs `null` as string "null" and exits 0. Use `jq -re` to fail on null/false:

| Scenario | Flag | Behavior |
|----------|------|----------|
| Null acceptable | `-r` | Outputs "null", exit 0 |
| Null is error | `-re` | Exits 1 on null/false |

```bash
# Fails pipeline if .value is null
value=$(echo "$json" | jq -re '.value')
```

## Reference

If implementing CLI argument parsing:
  Read: `.claude/rules/.reference/bash/argument-parsing.md`

If implementing parallel execution:
  Read: `.claude/rules/.reference/bash/parallel-execution.md`
