---
paths:
  - "files/home/kai/.config/claudex/config/agents/**/*.md"
---

* Investigate existing agents to extract conventions before creating new ones
* Re-run `bin/sync-agent-files.sh` after editing `files/home/kai/.config/claudex/config/agents/` so the tracked outputs under `files/home/kai/.codex/`, `files/home/kai/.gemini/`, and `files/home/kai/.config/opencode/` do not drift from their source
* Frontmatter requires `name` and `description`, and `tools` on any agent that has to write a file - omitting the field diverges rather than defaulting, since Codex emits `sandbox_mode = "read-only"` while opencode emits no permission block at all, leaving the same agent read-only in one tool and unrestricted in the other
* Use `ultrathink.` after `# Agent Instructions` for complex analysis tasks
* Include `## Input` section when agent needs external context (optional)
* Agents return findings; caller applies the Verification and Feedback procedures from CLAUDE.general.md
* Restate restrictions from `tools` in the body because Codex cannot reproduce a per-agent tool allow-list, Gemini omits scoped shell grants rather than broadening them, and Gemini subagents cannot delegate recursively
* Name the file after the invocation name you want; Gemini sanitizes the flattened file name, while Codex uses the frontmatter `name` when present

## Agent Structure

| Section | Purpose |
|---------|---------|
| `# Agent Instructions` | Section header (no description needed - use Objectives) |
| `ultrathink.` | Signal for extended reasoning |
| `## Objectives` | Bullet point with high-level goal (single item) |
| `## Process` | Numbered list of workflow steps |
| `## Important` | Bullet point list of constraints and guidelines |
| `## Input` | External context (optional) |

### Input Section Format

Use one format per agent, not mixed:

| Input Type | Format | Example |
|------------|--------|---------|
| Command output | `` !`command` `` syntax | `` !`git diff` `` |
| Prompt context | Descriptive text | "The following change summary will be provided:" |

## Conversion

| Output | `tools` allow-list | Body |
|--------|--------------------|------|
| `files/home/kai/.codex/agents/{name}.toml` | no per-tool equivalent; agents without an edit tool receive `sandbox_mode = "read-only"` | JSON-escaped as `developer_instructions` |
| `files/home/kai/.gemini/agents/{name}.md` | built-in and MCP tools are translated; scoped shell grants are omitted rather than broadened | verbatim |
| `files/home/kai/.config/opencode/agents/{name}.md` | `edit` / `bash` / `webfetch` deny entries derived from whole tokens, with `Bash(cmd:*)` and `Bash(cmd)` becoming a per-command `bash` map | verbatim |

## Reference

If creating a review agent that reads the diff directly:
  Read: `files/home/kai/.config/claudex/config/agents/final-review.md`

If creating a review agent that receives a change summary:
  Read: `files/home/kai/.config/claudex/config/agents/rules-review.md`

If creating a cleanup agent that reads the diff directly:
  Read: `files/home/kai/.config/claudex/config/agents/code-cleanup.md`

If creating a cleanup agent that receives a change summary:
  Read: `files/home/kai/.config/claudex/config/agents/file-cleanup.md`

If creating a verification agent:
  Read: `files/home/kai/.config/claudex/config/agents/verification.md`
