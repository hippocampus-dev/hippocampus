---
paths:
  - "**/README.md"
---

* Use kebab-case directory name as title: `# {directory-name}`
* Add `<!-- TOC -->` as a pair of markers after the title; TOC content listing all headings (`#` through `#####`) goes between them using `*` list markers with 2-space indentation per level
* Write one-line description starting with the kebab-case directory name: `{directory-name} is a {brief description}.`
* Optional sections follow fixed order: `## Features` → `## Requirements` → `## Usage` → `## Development` → `## Deployment`
* `## Features` may use `- [x]`/`- [ ]` checklist for implementation status
* `## Development` is required when a Makefile exists, with `$ make dev` as primary command
* Only include environment variables in Development that are required to run
* Use appropriate language identifiers in code blocks (`sh`, `bash`, `go`, `rust`, `python`, `yaml`); use `bash` when code contains bash-specific syntax

## Common Format

```markdown
# {directory-name}

<!-- TOC -->
<!-- TOC -->

{directory-name} is a {brief description}.

## Features

- Feature 1

## Usage

\`\`\`sh
$ command example
\`\`\`

## Development

\`\`\`sh
$ export REQUIRED_ENV=<value>
$ make dev
\`\`\`
```
