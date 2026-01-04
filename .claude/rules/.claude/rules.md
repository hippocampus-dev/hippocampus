---
paths:
  - ".claude/rules/**/*.md"
---

* Document patterns actually used in the project, not general best practices
* Investigate existing files to extract conventions (structure, naming, ordering)
* Keep rules concise - move specific patterns to `.reference/`
* Use specific `paths` patterns for auto-discovery

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

