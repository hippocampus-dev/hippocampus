---
paths:
  - "files/home/kai/.config/claudex/config/commands/**/*.md"
---

* Investigate existing commands to extract conventions before creating new ones
* Re-run `bin/sync-agent-files.sh` after editing `files/home/kai/.config/claudex/config/commands/` so the tracked outputs under `files/home/kai/.codex/`, `files/home/kai/.gemini/`, and `files/home/kai/.config/opencode/` do not drift from their source
* Frontmatter requires `description` field
* Add `allowed-tools` only when external tools (Bash, etc.) are needed
* Use `!` syntax for inline bash to show current state (e.g., `!`git branch --show-current``)
* End procedural commands with `## Instructions` using numbered steps
* `description` survives every conversion and `argument-hint` survives in Codex; `allowed-tools` has no command-file equivalent
* Inline bash written as `` !`cmd` `` is rewritten to Gemini's `!{cmd}` form and to an explicit run-command instruction for Codex
* `$ARGUMENTS` is rewritten to `{{args}}` for Gemini and remains `$ARGUMENTS` for Codex and opencode

## Command Types

| Type | Example | Structure |
|------|---------|-----------|
| Tool command | `git/weekly-report.md` | `allowed-tools` + `!` syntax + `## Instructions` |
| Persona command | `mimicry/kent-beck.md` | `# UPPERCASE HEADERS` + methodology + `# EXAMPLE WORKFLOW` |
| Simple delegation | `sop.md` | Description + single instruction |

## Reference

If creating a tool command:
  Read: `files/home/kai/.config/claudex/config/commands/git/weekly-report.md`

If creating a persona command:
  Read: `files/home/kai/.config/claudex/config/commands/mimicry/kent-beck.md`
