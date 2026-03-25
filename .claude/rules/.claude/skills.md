---
paths:
  - ".claude/skills/**/*.md"
---

* Document patterns actually used in the project, not general best practices
* Investigate existing files to extract conventions (structure, naming, ordering)
* SKILL.md contains core principles for all cases (max 500 lines)
* `reference/` contains detailed examples for specific patterns
* Description must be specific with keywords for auto-discovery

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
└── reference/
    └── specific-case.md  # Only loaded when that pattern is needed
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

When a user's prompt contains any keyword, the skill is suggested automatically.

### Keyword Selection

| Prefer | Avoid |
|--------|-------|
| Tool/command names (`kubectl`, `playwright`, `css`) | Generic terms (`javascript`, `test`, `log`) |
| Domain-specific terms (`deployment`, `pod`, `e2e`) | Ambiguous terms (`run`, `check`, `fix`) |
| File extensions/formats (`html`, `yaml`) | Common verbs (`create`, `update`, `delete`) |

Generic keywords cause false positives across unrelated prompts. Choose keywords that strongly signal the specific skill domain.

## SKILL.md Heading Hierarchy

Use `##` as the top-level heading in SKILL.md files:

| Level | Use |
|-------|-----|
| `##` | Main topic |
| `###` | Subtopic within a main topic |
| `####` | Detail within a subtopic |

See `.claude/rules/.reference/.claude/skills/format.md` for examples and additional formatting guidelines.

## Reference

If writing skill format details:
  Read: `.claude/rules/.reference/.claude/skills/format.md`
