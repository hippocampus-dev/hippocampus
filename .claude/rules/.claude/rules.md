---
paths:
  - ".claude/rules/**/*.md"
---

* Investigate existing files to extract conventions (structure, naming, ordering)
* Keep rules concise - move specific patterns to `.reference/`
* Use specific `paths` patterns for auto-discovery
* Do not hardcode dynamic data (used keys, current values) - instruct to read source files instead
* Match granularity of related items in tables (e.g., if one cell has conditions, others should too)
* Use heading levels to reflect logical hierarchy between topics (e.g., `##` parent with `###` children)

## When to Create Rules

| Create rule | Do NOT create rule |
|-------------|-------------------|
| Guideline for how to approach problems | Implementation details of a specific solution |
| Multiple valid approaches exist and one is preferred | Only one obvious way to do it |
| Easy to make mistakes without guidance | Standard/general best practices documented elsewhere |
| Enforces project-specific consistency | Patterns that naturally emerge |
| Tooling has project-specific behavior differing from defaults | Standard language/format behavior documented elsewhere |

Rules should document "how to do things" (guidelines), not "how something was done" (implementation details). Specific solutions belong in the relevant `CLAUDE.md`.

## Structure

```
.claude/rules/
├── {technology}.md           # File type rules (paths: "**/*.ext")
├── {directory-path}.md       # Directory-specific rules (paths: "{directory-path}/**")
├── {directory-path}/
│   └── {subdirectory}.md     # Nested directory rules
└── .reference/
    └── {topic}/
        └── {specific-case}.md  # Detailed patterns
```

## File Format

```yaml
---
paths:
  - "**/*.rs"
---

* Always-applicable rule 1
* Always-applicable rule 2
* Copy existing file (e.g., `path/to/example`) as template

## {Topic}

| Key | Value |
|-----|-------|

## Reference

If {condition}:
  Read: `.claude/rules/.reference/{topic}/{pattern}.md`
```

| Element | Purpose |
|---------|---------|
| Top-level bullets | Always-applicable rules (no plain text before bullets) |
| `## {Topic}` | Tables with structured reference data |
| `## Reference` | Read: links to detailed patterns |

## .reference/ File Format

```markdown
# Pattern Name

Description of when to use this pattern.

## When to Use
## Example
Copy from: `path/to/example`
## Files
## Key Modifications
```

`.reference/` files can use additional sections (`## Example`, `## Files`, etc.) not allowed in regular rule files.

## Directory-Specific Rules

Place in: `.claude/rules/{directory-path}.md`

| paths | File location |
|-------|---------------|
| `.github/workflows/**` | `.github/workflows.md` |
| `cluster/manifests/**` | `cluster/manifests.md` |
| `cluster/manifests/argocd-applications/**` | `cluster/manifests/argocd-applications.md` |
| `.claude/skills/**` | `.claude/skills.md` |
| `.claude/rules/**` | `.claude/rules.md` |

## Reference

Move to `.reference/` when:
* Information applies only to a specific pattern (e.g., Go-specific Dockerfile rules)
* The pattern is not needed for every use case of the parent rule

Example: `dockerfile.md` has common rules, `.reference/dockerfile/go.md` has Go-specific patterns.

