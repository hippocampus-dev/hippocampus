# Observability Setup

How to set up logging and tracing for Python services.

## When to Use context_logging

| Service Type | Logging | Reason |
|--------------|---------|--------|
| API services (FastAPI) | `context_logging` + OpenTelemetry | Distributed tracing needed |
| Slack bots | `context_logging` + OpenTelemetry | Event correlation needed |
| Batch jobs / CronJobs | Standard `logging` | Single task, no tracing needed |
| Queue workers | Standard `logging` | Sequential processing |

## context_logging Setup

Create `{project}/context_logging.py`:

```python
import collections.abc
import logging
import typing

import opentelemetry.trace


class ContextLogger(logging.LoggerAdapter):
    def process(
        self,
        msg: typing.Any,
        kwargs: collections.abc.MutableMapping[str, typing.Any],
    ) -> tuple[typing.Any, collections.abc.MutableMapping[str, typing.Any]]:
        context = opentelemetry.trace.get_current_span().get_span_context()
        extra = {
            "traceid": opentelemetry.trace.format_trace_id(context.trace_id),
            "spanid": opentelemetry.trace.format_span_id(context.span_id),
        }

        if "extra" in kwargs:
            kwargs["extra"] = {**kwargs["extra"], **extra}
        else:
            kwargs["extra"] = extra

        return msg, kwargs


def getLogger(name: str | None = None) -> logging.LoggerAdapter:
    return ContextLogger(logging.getLogger(name))
```

## Telemetry Module

Create `{project}/telemetry.py`:

```python
import opentelemetry.metrics
import opentelemetry.trace

import {project}.context_logging

tracer = opentelemetry.trace.get_tracer("{project}")
meter = opentelemetry.metrics.get_meter("{project}")
logger = {project}.context_logging.getLogger("{project}")
```

## Required Dependencies

```toml
[project]
dependencies = [
    "opentelemetry-api==1.24.0",
    "opentelemetry-sdk==1.24.0",
    "opentelemetry-exporter-otlp==1.24.0",
    "python-json-logger==2.0.7",
]
```
