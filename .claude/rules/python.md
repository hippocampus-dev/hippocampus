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
* FastAPI handlers performing expensive non-streaming downstream calls (LLM/embedding APIs, S3 round-trips): wrap the expensive section with `cancel_on_disconnect(request)` so client disconnect cancels the task; copy `cancellation.py` from an existing adopter (e.g., `cluster/applications/embedding-gateway/embedding_gateway/cancellation.py`)
* Do NOT wrap SSE/streaming handlers that use `sse_buffer` resumability — the producer task is intentionally detached via `_track_background_task`
* Long-lived processes driving Playwright (exporters, renderers): do DOM traversal inside a single `page.evaluate()` rather than walking `ElementHandle`s — memory growth scales with the number of Playwright API calls per page, and disposing handles, relaunching the browser or recycling the `sync_playwright()` context do not reclaim it (example: `cluster/applications/realtime-search-exporter/main.py`)
* Do NOT add periodic `gc.collect()`/`malloc_trim()` against that growth — the client captures a Python stack on every call with no option to disable it, so fewer API calls only lowers the rate and they reclaim nothing on top

## Reference

If setting up observability for a new service:
  Read: `.claude/reference/python/observability.md`

If writing tests:
  Read: `.claude/reference/python/testing.md`

If implementing HTTP client with aiohttp or httpx:
  Read: `.claude/reference/python/http-client.md`
