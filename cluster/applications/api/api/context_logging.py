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
