---
paths:
  - ".claude/commands/**/*.md"
---

* Investigate existing commands to extract conventions before creating new ones
* Re-run `bin/sync-agent-files.sh` after editing `.claude/commands/` so the tracked mirrors under `.gemini/commands/` and `.opencode/commands/` do not drift from their source
* Project commands have no project-scoped Codex custom-prompt equivalent; use a project skill when Codex also needs the workflow
* `description` survives every conversion; `argument-hint` also survives in global Codex custom prompts, while `allowed-tools` has no command-file equivalent
* Gemini converts `$ARGUMENTS` to `{{args}}` and `` !`command` `` to `!{command}`; opencode accepts both Claude forms unchanged

## Conversion

| Output | Frontmatter kept | Body |
|--------|------------------|------|
| `.gemini/commands/{name}.toml` | `description` | TOML-escaped, with `$ARGUMENTS` rewritten to `{{args}}` and shell injection rewritten to `!{...}` |
| `.opencode/commands/{name}.md` | `description` | verbatim, so `$ARGUMENTS` reaches it unrewritten |

## Reference

If creating a command:
  Read: `.claude/commands/explain-diff.md`
