---
paths:
  - ".claude/agents/**/*.md"
---

* Investigate the agents under `files/home/kai/.config/claudex/config/agents/` for frontmatter and tooling conventions before creating new ones, and take the body convention from the bullet below rather than from them
* Re-run `bin/sync-agent-files.sh` after editing `.claude/agents/` so the tracked mirrors under `.codex/agents/`, `.gemini/agents/`, and `.opencode/agents/` do not drift from their source
* Write the body as prose; the `# Agent Instructions` / `## Objectives` / `## Process` / `## Important` structure applies under `files/home/kai/.config/claudex/config/agents/` only
* Restate restrictions from `tools` in the body because Codex cannot reproduce a per-agent tool allow-list, Gemini cannot represent scoped `Bash(command:*)` entries in an agent file, and Gemini subagents cannot delegate recursively
* Name the file after the invocation name you want; Gemini derives its sanitized name from the file name, while Codex uses the frontmatter `name` when present

## Conversion

| Output | `tools` allow-list | Body |
|--------|--------------------|------|
| `.codex/agents/{name}.toml` | no per-tool equivalent; agents without an edit tool receive `sandbox_mode = "read-only"` | JSON-escaped as `developer_instructions` |
| `.gemini/agents/{name}.md` | built-in and MCP tools are translated; scoped shell grants are omitted rather than broadened | verbatim |
| `.opencode/agents/{name}.md` | `edit` / `bash` / `webfetch` deny entries derived from whole tokens, with `Bash(cmd:*)` and `Bash(cmd)` becoming a per-command `bash` map | verbatim |
