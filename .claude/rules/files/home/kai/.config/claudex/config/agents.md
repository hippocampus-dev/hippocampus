---
paths:
  - "files/home/kai/.config/claudex/config/agents/**/*.md"
---

* Investigate existing agents to extract conventions before creating new ones
* Re-run `bin/sync-agent-files.sh` after editing `files/home/kai/.config/claudex/config/agents/` so the tracked outputs under `files/home/kai/.codex/`, `files/home/kai/.gemini/`, and `files/home/kai/.config/opencode/` do not drift from their source
* Frontmatter requires `name`, `description`, and `tools` fields
* Use `ultrathink.` after `# Agent Instructions` for complex analysis tasks
* Include `## Input` section when agent needs external context (optional)
* Agents return findings; caller applies the Verification and Feedback procedures from CLAUDE.general.md
* Restate restrictions from `tools` in the body because Codex cannot reproduce a per-agent tool allow-list and Gemini omits scoped shell grants rather than broadening them
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

## Reference

If creating a review agent:
  Read: `files/home/kai/.config/claudex/config/agents/final-review.md`

If creating a cleanup agent:
  Read: `files/home/kai/.config/claudex/config/agents/code-cleanup.md`

If creating a verification agent:
  Read: `files/home/kai/.config/claudex/config/agents/verification.md`
