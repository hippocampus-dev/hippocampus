---
paths:
  - ".claude/commands/**/*.md"
---

* Investigate existing commands to extract conventions before creating new ones
* Re-run `bin/sync-agent-files.sh` after editing `.claude/commands/` so the tracked mirrors under `.gemini/commands/`, `.agents/commands/`, and `.opencode/commands/` do not drift from their source
* Project commands have no project-scoped Codex custom-prompt equivalent; use a project skill when Codex also needs the workflow
* `description` survives every conversion; `argument-hint` also survives in global Codex custom prompts, while `allowed-tools` has no command-file equivalent
* Gemini and Antigravity convert `$ARGUMENTS` to `{{args}}` and `` !`command` `` to `!{command}`; opencode accepts both Claude forms unchanged
* `.claude/rules/files/home/kai/.config/claudex/config/commands.md`'s `Spell $ARGUMENTS only where the argument's value belongs` holds here too - `bin/sync-agent-files.sh` runs that same global `sed` over this directory

## Conversion

| Output | Frontmatter kept | Body |
|--------|------------------|------|
| `.gemini/commands/{name}.toml` | `description` | TOML-escaped, with `$ARGUMENTS` rewritten to `{{args}}` and shell injection rewritten to `!{...}` |
| `.agents/commands/{name}.toml` | `description` | identical to the Gemini output; Antigravity loads a command as a skill |
| `.opencode/commands/{name}.md` | `description` | verbatim, so `$ARGUMENTS` reaches it unrewritten |

## Reference

If creating a command:
  Read: `.claude/commands/explain-diff.md`
