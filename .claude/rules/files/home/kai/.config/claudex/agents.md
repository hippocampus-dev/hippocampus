---
paths:
  - "files/home/kai/.config/claudex/agents/**/*.md"
---

* Investigate existing agents to extract conventions before creating new ones
* Frontmatter requires `name`, `description`, and `tools` fields
* Use `ultrathink.` after `# Agent Instructions` for complex analysis tasks
* Include `## Input` section when agent needs external context (optional)
* Use `## User Selection` when agent should present choices and apply fixes interactively

## Agent Structure

| Section | Purpose |
|---------|---------|
| `# Agent Instructions` | Brief description of what the agent does |
| `ultrathink.` | Signal for extended reasoning (optional) |
| `## Objectives` | Numbered list of goals |
| `## Process` | Step-by-step workflow |
| `## Important` | Constraints and guidelines |
| `## User Selection` | How to present findings via AskUserQuestion (optional) |
| `## Input` | External context: `!` syntax for commands or description for prompt input (optional) |

## Interactive Review Pattern

When an agent reviews code and lets user select which issues to fix:

1. Add `AskUserQuestion`, `Edit`, `MultiEdit` to tools
2. Add `## User Selection` describing how to present findings
3. If ≤4 findings: present each as option with `multiSelect: true`
4. If >4 findings: group by severity (Critical/High, Medium, Low, Skip)
5. Apply remediation for selected items using Edit/MultiEdit

## Reference

If creating a review agent with actionable findings:
  Read: `files/home/kai/.config/claudex/agents/final-review.md`

If creating a cleanup agent:
  Read: `files/home/kai/.config/claudex/agents/code-cleanup.md`

If creating a verification agent:
  Read: `files/home/kai/.config/claudex/agents/verification.md`
