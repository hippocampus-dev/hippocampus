---
paths:
  - ".claude/commands/**/*.md"
---

* Investigate existing commands to extract conventions before creating new ones
* Re-run `bin/sync-agent-files.sh` after editing `.claude/commands/` so the tracked mirrors under `.gemini/commands/`, `.agents/skills/`, and `.opencode/commands/` do not drift from their source
* `description` survives every conversion; `argument-hint` and `disable-model-invocation` also survive in `.agents/skills/` (`disable-model-invocation: true` additionally writes `agents/openai.yaml` with `allow_implicit_invocation: false` for Codex), while `allowed-tools` has no equivalent
* Gemini converts `$ARGUMENTS` to `{{args}}` and `` !`command` `` to `!{command}`; `.agents/skills/` rewrites `$ARGUMENTS` to prose and `` !`command` `` to an explicit run-command instruction for Codex and Antigravity; opencode accepts both Claude forms unchanged
* `.claude/rules/files/home/kai/.config/claudex/config/commands.md`'s `Add disable-model-invocation: true` holds here too - Claude Code lists a project command in the same model-invocable roster as a global one
* `.claude/rules/files/home/kai/.config/claudex/config/commands.md`'s `Spell $ARGUMENTS only where the argument's value belongs` holds here too - `bin/sync-agent-files.sh` runs that same global `sed` over this directory
* `.claude/rules/files/home/kai/.config/claudex/config/commands.md`'s `Write a keyword an argument-hint expects verbatim bare` holds here too - `.claude/commands/bump.md` carries an `argument-hint` and it reaches the same `/` autocomplete menu

## Conversion

| Output | Frontmatter kept | Body |
|--------|------------------|------|
| `.gemini/commands/{name}.toml` | `description` | TOML-escaped, with `$ARGUMENTS` rewritten to `{{args}}` and shell injection rewritten to `!{...}` |
| `.agents/skills/{name}/SKILL.md` | `name`, `description`, `argument-hint`, `disable-model-invocation` (plus `agents/openai.yaml` when `disable-model-invocation: true`) | `$ARGUMENTS` rewritten to prose and shell injection rewritten to an explicit run-command instruction for Codex and Antigravity |
| `.opencode/commands/{name}.md` | `description` | verbatim, so `$ARGUMENTS` reaches it unrewritten |

## Reference

If creating a command:
  Read: `.claude/commands/explain-diff.md`
