---
paths:
  - "**/*.go"
---

* Implement tests with [TableDrivenTest](https://go.dev/wiki/TableDrivenTests) pattern for multiple test cases
* Wrap errors with `xerrors.Errorf` to get complete stack traces
* Declare function arguments individually like `(a string, b string)` instead of `(a, b string)`
* Use `Args` struct + `DefaultArgs()` pattern for CLI configuration

## Reference

If implementing CLI configuration:
  Read: `.claude/rules/.reference/go/configuration.md`
