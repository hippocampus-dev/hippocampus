---
paths:
  - "**/Makefile"
---

* Use `@` prefix to suppress command echo
* All targets are `.PHONY`
* Use `$(shell ...)` for dynamic values
* End `all` target with `@` (empty success command)

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
  Read: `.claude/rules/.reference/makefile/python.md`
  Read: `.claude/rules/.reference/makefile/rust.md`
