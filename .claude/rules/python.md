---
paths:
  - "**/*.py"
---

* Follow PEP 8 style guide
* Use full module paths like `collections.abc.Mapping` instead of `from collections.abc import Mapping`
* API services and Slack bots: Use `context_logging` + OpenTelemetry
* Batch jobs and workers: Use `pythonjsonlogger.jsonlogger.JsonFormatter` (JSON in production, plain `logging.Formatter` when `is_debug()`); copy the `JsonFormatter` + handler block from an existing worker `main.py`
* Settings: use `pydantic_settings.BaseSettings` with `SettingsConfigDict(extra="allow", env_file=".env")`, `is_debug()` (`sys.prefix != sys.base_prefix`) and `convert_log_level()` methods; copy from an existing `{package}/settings.py`
* Use `model.py` (singular) for Pydantic model files, not `models.py`

## Reference

If setting up observability for a new service:
  Read: `.claude/reference/python/observability.md`

If implementing HTTP client with aiohttp or httpx:
  Read: `.claude/reference/python/http-client.md`
