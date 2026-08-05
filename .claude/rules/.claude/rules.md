---
paths:
  - ".claude/rules/**/*.md"
  - ".claude/reference/**/*.md"
---

* Investigate existing files to extract conventions (structure, naming, ordering)
* Keep rules concise - move specific patterns to `.claude/reference/`
* Use specific `paths` patterns for auto-discovery - put an item in the rule file whose `paths` match only the files it constrains, and add a nested file mirroring the target path instead of appending to a broader ancestor
* Enumerate and classify every file the rule's `paths` match before stating an exclusion as a criterion rather than a named list - the enumeration fixes the denominator two reviewers would otherwise pick differently, and it must come from the `paths` glob itself rather than from a content filter such as a shebang, since such a filter drops the very files the rule overlooks
* Do not hardcode dynamic data (used keys, current values) - instruct to read source files instead
* Match granularity of related items in tables (e.g., if one cell has conditions, others should too)
* Use heading levels to reflect logical hierarchy between topics (e.g., `##` parent with `###` children)
* State a convention in exactly one place - put it in `files/home/kai/.config/claudex/config/CLAUDE*.md` when it must hold in every repository, otherwise in `.claude/rules/`, whose generated `AGENTS.md` mirrors already carry it to every other tool inside this checkout - which tools honor it decides nothing, so a convention `bin/sync-agent-files.sh` strips from every other tool's `AGENTS.md` still belongs in `CLAUDE*.md` when its scope is global
* Re-run `bin/sync-agent-files.sh` after editing `.claude/rules/` so the directory-scoped `AGENTS.md` mirrors it generates do not drift from their source
* Check what already globs a directory before scoping a rule's `paths` to it - the mirror lands there as a real file, so every build step and copy that walks the directory silently gains an input; narrow the consumer's glob rather than the rule's `paths`, and where that directory is one the rule's own `paths` glob, the mirror returns as an input to the rule that produced it and no narrowing is available, so the rule must carry an explicit branch for it (`.claude/rules/bash.md`'s `## Header by script kind`)

## When to Create Rules

Before creating a rule, verify what is being asked and search `.claude/rules/` and `.claude/skills/` for the topic.
If unclear, clarify with the user first.

| Create rule | Do NOT create rule |
|-------------|-------------------|
| Guideline for how to approach problems | Implementation details of a specific solution |
| Multiple valid approaches exist and one is preferred | Only one obvious way to do it |
| Easy to make mistakes without guidance | Standard/general best practices documented elsewhere |
| Enforces project-specific consistency | Patterns that naturally emerge |
| Tooling has project-specific behavior differing from defaults | Standard language/format behavior documented elsewhere |
| Fills a gap not covered by existing rules or skills | Duplicates or contradicts an existing rule or skill |
| Two or more in-tree files already follow the convention | Fewer than two in-tree files follow it |

The count applies to a convention generalized from files, not to a trap confirmed against the tool's own behavior - its source, its documentation, or a live reproduction (see `## Verifying Changes Against Live State` in `.claude/rules/cluster/manifests.md`).

Rules should document "how to do things" (guidelines), not "how something was done" (implementation details).

## Structure

```
.claude/
├── rules/
│   ├── {technology}.md           # File type rules (paths: "**/*.ext")
│   ├── {directory-path}.md       # Directory-specific rules (paths: "{directory-path}/**")
│   └── {directory-path}/
│       └── {subdirectory}.md     # Nested directory rules
└── reference/
    └── {topic}/
        └── {specific-case}.md    # Detailed patterns (not auto-loaded)
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
  Read: `.claude/reference/{topic}/{pattern}.md`
```

| Element | Purpose |
|---------|---------|
| Top-level bullets | One always-applicable rule per bullet, with that rule's qualifiers and rationale attached - open a new bullet when a second independently-actionable norm appears (no plain text before bullets) |
| `## {Topic}` | Decision guidance tables (not current state listings) |
| `## Reference` | `Read:` links to detailed patterns, or the answer inline when it is too short to earn its own file |

## Content Validation

Before adding content, verify it passes both checks:

| Check | If No |
|-------|-------|
| Project-specific? (not covered by external docs) | Remove |
| Prescriptive? (requirement/choice/warning, not current state) | Remove |

## .claude/reference/ File Format

```markdown
# Pattern Name

Description of when to use this pattern.

## When to Use
## Example
Copy from: `path/to/example`
## Files
## Key Modifications
```

`.claude/reference/` files can use additional sections (`## Example`, `## Files`, etc.) not allowed in regular rule files.

## Directory-Specific Rules

Place in: `.claude/rules/{directory-path}.md`

| paths | File location |
|-------|---------------|
| `.github/workflows/**` | `.github/workflows.md` |
| `cluster/manifests/**` | `cluster/manifests.md` |
| `cluster/manifests/argocd-applications/**` | `cluster/manifests/argocd-applications.md` |
| `.claude/skills/**` | `.claude/skills.md` |
| `.claude/rules/**` | `.claude/rules.md` |
| `.claude/reference/**` | `.claude/rules.md` |

## Reference

Move to `.claude/reference/` when:
* Information applies only to a specific pattern (e.g., Go-specific Dockerfile rules)
* The pattern is not needed for every use case of the parent rule
* Every other file reaches the pattern through an interface (e.g., a reusable workflow) instead of writing it, so the author of a new file never reproduces it

A `## Reference` pointer only fires for a condition the author can already name, so it cannot carry a trap the author would never think to look up.
Keep such content in the rule against the conditions above - the pattern is written inline as part of something ordinary and a deviation raises no error where the mistake is made - however few files do so today.

Example: `dockerfile.md` has common rules, `.claude/reference/dockerfile/go.md` has Go-specific patterns.

