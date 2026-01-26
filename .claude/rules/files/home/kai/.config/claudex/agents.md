---
paths:
  - "files/home/kai/.config/claudex/agents/**/*.md"
---

* Investigate existing agents to extract conventions before creating new ones
* Frontmatter requires `name`, `description`, and `tools` fields
* Use `ultrathink.` after `# Agent Instructions` for complex analysis tasks
* Include `## Input` section when agent needs external context (optional)
* Agents return findings; caller applies Pattern Validation and User Selection from CLAUDE.important.md

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
  Read: `files/home/kai/.config/claudex/agents/final-review.md`

If creating a cleanup agent:
  Read: `files/home/kai/.config/claudex/agents/code-cleanup.md`

If creating a verification agent:
  Read: `files/home/kai/.config/claudex/agents/verification.md`
