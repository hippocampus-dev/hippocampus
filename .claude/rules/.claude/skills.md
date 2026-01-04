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
---
```

## Reference

If writing skill format details:
  Read: `.claude/rules/.reference/.claude/skills/format.md`
