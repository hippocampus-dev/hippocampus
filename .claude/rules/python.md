---
paths:
  - "**/*.py"
---

* Follow PEP 8 style guide
* Use full module paths like `collections.abc.Mapping` instead of `from collections.abc import Mapping`
* API services and Slack bots: Use `context_logging` + OpenTelemetry
* Batch jobs and workers: Use standard `logging` only
* Use `model.py` (singular) for Pydantic model files, not `models.py`

## Reference

If setting up observability for a new service:
  Read: `.claude/rules/.reference/python/observability.md`

If implementing HTTP client with aiohttp or httpx:
  Read: `.claude/rules/.reference/python/http-client.md`
