---
paths:
  - "files/home/kai/.config/claudex/config/agents/**/*.md"
---

* Investigate existing agents to extract conventions before creating new ones
* Re-run `bin/sync-agent-files.sh` after editing `files/home/kai/.config/claudex/config/agents/` so the tracked outputs under `files/home/kai/.codex/`, `files/home/kai/.gemini/`, and `files/home/kai/.config/opencode/` do not drift from their source
* Frontmatter requires `name` and `description`, and `tools` on any agent that has to write a file - omitting the field diverges rather than defaulting, since Codex emits `sandbox_mode = "read-only"` while opencode emits no permission block at all, leaving the same agent read-only in one tool and unrestricted in the other
* Set `effort` in the frontmatter of every agent whose body follows `## Agent Structure`, to `low`, `medium`, `high`, `xhigh` or `max` rather than the integer the field also accepts, and keep `ultrathink.` after `# Agent Instructions` on exactly those set to `high` or above - `effort` alone sets the depth Claude Code asks the API for, while `ultrathink.` adds an in-context request that `bin/sync-agent-files.sh` copies into every generated body, so a mismatched pair asks those bodies for depth the frontmatter withholds or leaves them without the depth it grants
* Choose the level by an agent's worst outcome rather than by how long it runs - an agent that only reports is bounded by a miss the next run still catches, while one that acts on its findings is bounded by the action it gets wrong, which no later run undoes
* Lowering `effort` shortens a batch of agents launched together only down to its highest-set member, since an agent's wall time tracks the tokens it generates - `files/home/kai/.config/claudex/config/CLAUDE.important.md`'s task-creation list names which agents are launched together
* Include `## Input` section when agent needs external context (optional)
* Every field an agent's `## Input` names has to be one the change summary in `files/home/kai/.config/claudex/config/CLAUDE.important.md`'s task-creation list carries, and the agent has to be launched from the item that hands that summary out - a field the list does not carry arrives empty, and an agent launched from another item gets nothing where it expects a summary
* Agents return findings; caller applies the Verification and Feedback procedures from CLAUDE.general.md
* Restate restrictions from `tools` in the body because Codex cannot reproduce a per-agent tool allow-list, Gemini omits scoped shell grants rather than broadening them, and Gemini subagents cannot delegate recursively
* Name the file after the invocation name you want; Gemini sanitizes the flattened file name, while Codex uses the frontmatter `name` when present

## Agent Structure

| Section | Purpose |
|---------|---------|
| `# Agent Instructions` | Section header (no description needed - use Objectives) |
| `ultrathink.` | In-context request for deeper reasoning (only at `effort` `high` or above) |
| `## Objectives` | Bullet point with high-level goal (single item) |
| `## Process` | Numbered list of workflow steps |
| `## Important` | Bullet point list of constraints and guidelines |
| `## Input` | External context (optional) |

### Input Section Format

Use one format per agent, not mixed:

| Agent's findings must be | Format | Example |
|--------------------------|--------|---------|
| Attributable to the change under review | Descriptive text naming the change summary fields the agent needs | "The following change summary will be provided:" |
| Whatever the command reports at run time, whoever produced it | `` !`command` `` syntax naming the command whose output the agent needs | `` !`git diff` `` |

## Conversion

| Output | `tools` allow-list | Body |
|--------|--------------------|------|
| `files/home/kai/.codex/agents/{name}.toml` | no per-tool equivalent; agents without an edit tool receive `sandbox_mode = "read-only"` | JSON-escaped as `developer_instructions` |
| `files/home/kai/.gemini/agents/{name}.md` | built-in and MCP tools are translated; scoped shell grants are omitted rather than broadened | verbatim |
| `files/home/kai/.gemini/config/agents/{name}.md` | translated to the names listed at https://antigravity.google/docs/hooks; scoped shell grants are omitted, and the todo tools, `Skill`, `NotebookEdit` and MCP tools have no counterpart to translate to | verbatim |
| `files/home/kai/.config/opencode/agents/{name}.md` | `edit` / `bash` / `webfetch` deny entries derived from whole tokens, with `Bash(cmd:*)` and `Bash(cmd)` becoming a per-command `bash` map | verbatim |

## Reference

If creating a review agent that inspects the change itself:
  Read: `files/home/kai/.config/claudex/config/agents/final-review.md`

If creating a review agent that checks documentation or rules against the change:
  Read: `files/home/kai/.config/claudex/config/agents/rules-review.md`

If creating a cleanup agent that edits the changed files:
  Read: `files/home/kai/.config/claudex/config/agents/code-cleanup.md`

If creating a cleanup agent that removes files the session created:
  Read: `files/home/kai/.config/claudex/config/agents/file-cleanup.md`

If creating a verification agent:
  Read: `files/home/kai/.config/claudex/config/agents/verification.md`
