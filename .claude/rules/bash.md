---
paths:
  - "**/*.sh"
---

* Use `#!/usr/bin/env bash` as shebang (`#!/usr/bin/env -S bash -l` for login shell)
* Always include `set -e` at the top
* Use `[ ]` for conditionals (`[[ ]]` only when `<`, `>`, or `=~` is required)
* Quote variables like `"${var}"`
* Use `while IFS= read -r` for file path processing

## Reference

If implementing CLI argument parsing:
  Read: `.claude/rules/.reference/bash/argument-parsing.md`

If implementing parallel execution:
  Read: `.claude/rules/.reference/bash/parallel-execution.md`
