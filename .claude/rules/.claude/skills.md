---
paths:
  - ".claude/skills/**/*.md"
---

* Document patterns actually used in the project, not general best practices
* Write what changes the reader's next step, not why the environment it runs in was built that way - a fact carried as the qualifier on a step the reader takes breaks visibly once it stops holding, while a derivation of how that environment was assembled has no step depending on it and survives as a stale explanation
* Apply the project-specific check in `## Content Validation` of `.claude/rules/.claude/rules.md` to SKILL.md as well - a flag whose behaviour its own `--help` already documents fails it, while a trap confirmed against the tool's actual behaviour passes; the prescriptive check beside it does not carry over, since `## SKILL.md Content Guidelines` below admits overview tables of current state
* Investigate existing files to extract conventions (structure, naming, ordering)
* SKILL.md contains core principles for all cases (hard limit 500 lines)
* `reference/` contains detailed examples for specific patterns
* Name every supporting file under a skill directory from its SKILL.md by its skill-relative path - the skill offers no other entry point, so a file nothing names reaches no reader and its content is rediscovered from scratch
* Description must be specific with keywords for auto-discovery
* Before adding a skill, state in one sentence how its activation boundary differs from existing skills; consolidate into the existing skill when it does not differ, and encode the difference as description disambiguation when it does (see `.claude/reference/.claude/skills/format.md`)
* Re-run `bin/sync-agent-files.sh` after editing `.claude/skills/` so the tracked mirrors under `.codex/skills/`, `.gemini/skills/`, and `.agents/skills/` do not drift from their source; every supporting file under the skill directory is copied verbatim

## SKILL.md Content Guidelines

| Include in SKILL.md | Move to reference/ |
|---------------------|-------------------|
| Core workflow steps | Detailed query examples |
| Overview tables | Language-specific templates |
| Basic command syntax | Extended configuration samples |
| Decision flowcharts | Architecture diagrams |

Split when SKILL.md exceeds ~100 lines or contains pattern-specific details

## Structure

```
.claude/skills/{skill-name}/
├── SKILL.md              # Required: core principles for all cases
├── scripts/              # Optional: deterministic helpers
├── references/           # Optional: detailed documentation
└── assets/               # Optional: templates and static resources
```

## Metadata

```yaml
---
name: skill-name           # Required: lowercase, hyphens, max 64 chars
description: Description   # Required: max 1024 chars, specific keywords
keywords: keyword1, keyword2, キーワード  # Optional: comma-separated, for auto-activation
---
```

## Keywords (Auto-Activation)

The `keywords` field enables automatic skill suggestions via UserPromptSubmit hook.

| Language | Example Keywords |
|----------|------------------|
| English | playwright, browser, automation |
| Japanese | ブラウザ, 自動化, テスト |

A keyword made only of ASCII is matched at a word boundary and tolerates a trailing `s`, so `pod` fires on `pods` but not inside `podman`, while a keyword carrying any other character is matched anywhere in the prompt because Japanese has no word delimiter to anchor to - `matches()` in `files/home/kai/.config/claudex/config/hooks/UserPromptSubmit.sh` is what decides.

### Keyword Selection

| Prefer | Avoid |
|--------|-------|
| Tool/command names (`kubectl`, `playwright`, `css`) | Generic terms (`javascript`, `test`, `log`) |
| Domain-specific terms (`deployment`, `pod`, `e2e`) | Ambiguous terms (`run`, `check`, `fix`) |
| File extensions/formats (`html`, `yaml`) | Common verbs (`create`, `update`, `delete`) |

Generic keywords cause false positives across unrelated prompts.
Choose keywords that strongly signal the specific skill domain.

## SKILL.md Heading Hierarchy

Use `##` as the top-level heading in SKILL.md files:

| Level | Use |
|-------|-----|
| `##` | Main topic |
| `###` | Subtopic within a main topic |
| `####` | Detail within a subtopic |

See `.claude/reference/.claude/skills/format.md` for examples and additional formatting guidelines.

## Reference

If writing skill format details:
  Read: `.claude/reference/.claude/skills/format.md`
