---
paths:
  - "**/README.md"
---

* The repository root `README.md` additionally follows `root-readme.md`, which overrides conflicting items here
* Use kebab-case directory name as title: `# {directory-name}`
* Add `<!-- TOC -->` as a pair of markers after the title; TOC content listing all headings (`#` through `#####`) goes between them using `*` list markers with 2-space indentation per level
* Write one-line description starting with the kebab-case directory name: `{directory-name} is a {brief description}.`
* Update the matching Project Structure entry in the repository root `README.md` whenever a subdirectory `README.md`'s one-line description changes — that entry carries a condensed copy of it, and editing that `README.md` alone never triggers `root-readme.md`
* Optional sections follow fixed order: `## Features` → `## Requirements` → `## Usage` → `## Development` → `## Deployment`
* `## Features` may use `- [x]`/`- [ ]` checklist for implementation status
* `## Development` is required when a Makefile exists; show the Makefile's actual primary target — `$ make dev` when a `dev` target exists, otherwise the `.DEFAULT_GOAL` target
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
