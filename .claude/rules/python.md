---
paths:
  - "**/*.py"
---

* Follow PEP 8 style guide
* Use full module paths like `collections.abc.Mapping` instead of `from collections.abc import Mapping`
* API services and Slack bots: Use `context_logging` + OpenTelemetry
* Batch jobs and workers: Use standard `logging` only

## Reference

If setting up observability for a new service:
  Read: `.claude/rules/.reference/python/observability.md`
