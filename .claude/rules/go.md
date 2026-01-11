---
paths:
  - "**/*.go"
---

* Implement tests with [TableDrivenTest](https://go.dev/wiki/TableDrivenTests) pattern for multiple test cases
* Wrap errors with `xerrors.Errorf` to get complete stack traces
* Declare function arguments individually like `(a string, b string)` instead of `(a, b string)`
* Use `Args` struct + `DefaultArgs()` pattern for CLI configuration
* Use one-character receiver names (`c` for `*Client`, `d` for `*Dispatcher`, `h` for `*Handler`)

## Reference

If implementing CLI configuration:
  Read: `.claude/rules/.reference/go/configuration.md`

If implementing HTTP client:
  Read: `.claude/rules/.reference/go/http-client.md`

If writing tests:
  Read: `.claude/rules/.reference/go/testing.md`

If implementing admission webhook with container injection:
  Read: `.claude/rules/.reference/go/admission-webhook.md`
