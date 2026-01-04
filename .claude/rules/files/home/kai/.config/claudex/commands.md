---
paths:
  - "files/home/kai/.config/claudex/commands/**/*.md"
---

* Investigate existing commands to extract conventions before creating new ones
* Frontmatter requires `description` field
* Add `allowed-tools` only when external tools (Bash, etc.) are needed
* Use `!` syntax for inline bash to show current state (e.g., `!`git branch --show-current``)
* End procedural commands with `## Instructions` using numbered steps

## Command Types

| Type | Example | Structure |
|------|---------|-----------|
| Tool command | `git/weekly-report.md` | `allowed-tools` + `!` syntax + `## Instructions` |
| Persona command | `mimicry/kent-beck.md` | `# UPPERCASE HEADERS` + methodology + `# EXAMPLE WORKFLOW` |
| Simple delegation | `sop.md` | Description + single instruction |

## Reference

If creating a tool command:
  Read: `files/home/kai/.config/claudex/commands/git/weekly-report.md`

If creating a persona command:
  Read: `files/home/kai/.config/claudex/commands/mimicry/kent-beck.md`
