---
paths:
  - "**/Makefile"
---

* Use `@` prefix to suppress command echo
* All targets are `.PHONY`
* Use `$(shell ...)` for dynamic values
* End `all` target with `@` (empty success command)
* Make runs each recipe line in a separate shell — `trap` and background PID (`$!`) do not persist across lines. When `trap` is needed, extract the logic into a shell script

## Common Targets

| Target | Purpose |
|--------|---------|
| `make all` | Run all checks (fmt, lint, tidy, test) |
| `make fmt` | Format code |
| `make lint` | Lint and auto-fix |
| `make dev` | Start development mode |

## Reference

If writing a Makefile for a specific language:
  Read: `.claude/rules/.reference/makefile/go.md`
  Read: `.claude/rules/.reference/makefile/nodejs.md`
  Read: `.claude/rules/.reference/makefile/python.md`
  Read: `.claude/rules/.reference/makefile/rust.md`

If writing a Makefile for an ext-proc application:
  Read: `.claude/rules/.reference/makefile/ext-proc.md`
