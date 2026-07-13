# SKILL.md Format Details

Detailed formatting guidelines for specific cases.

## Description Guidelines

The description determines when Claude auto-applies the skill.

**Good** (specific, keyword-rich):
```yaml
description: Kubernetes manifest conventions including Kustomize overlays, secure pod defaults. Use when writing Deployments, Services, or kustomization.yaml.
```

**Good** (with disambiguation for overlapping domains):
```yaml
description: Operations for local container stacks defined in docker-compose.yaml. For cluster Grafana/Prometheus (via kubectl or cluster/manifests/), use kubernetes-operations instead.
```

**Bad** (vague):
```yaml
description: Helps with Kubernetes stuff
```

When skills have overlapping keywords (e.g., both handle grafana, prometheus), add explicit disambiguation: "For X scenario, use Y skill instead."

## Heading Hierarchy

Use `##` as the top-level heading in SKILL.md. Use heading levels to reflect logical hierarchy:

| Level | Use |
|-------|-----|
| `##` | Main topic |
| `###` | Subtopic within a main topic |
| `####` | Detail within a subtopic |

Example:

```markdown
## Debugging

### Observability Investigation (via Grafana)
Details about using Grafana...

### Pod Direct Investigation (via kubectl)
Details about kubectl debugging...

#### Ephemeral Container
Details about ephemeral containers...
```

Parent topics organize major workflows, children group related decision points or techniques within that workflow.

## Linking to Reference

Use relative links from SKILL.md:
```markdown
* For Go-specific patterns - see [Go](reference/go.md)
* For Python-specific patterns - see [Python](reference/python.md)
```

## Reference File Structure

```markdown
# Specific Pattern Name

When this pattern applies.

## Key Points

* Pattern-specific rule 1
* Pattern-specific rule 2

## Template

\`\`\`yaml
key: {placeholder}
\`\`\`
```
