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


def getLogger(name: str | None = None) -> logging.LoggerAdapter:  # noqa
    return ContextLogger(logging.getLogger(name))


root = ContextLogger(logging.root)


def critical(msg, *args, **kwargs):
    if len(logging.root.handlers) == 0:
        logging.basicConfig()
    root.critical(msg, *args, **kwargs)


def error(msg, *args, **kwargs):
    if len(logging.root.handlers) == 0:
        logging.basicConfig()
    root.error(msg, *args, **kwargs)


def exception(msg, *args, exc_info=True, **kwargs):
    error(msg, *args, exc_info=exc_info, **kwargs)


def warning(msg, *args, **kwargs):
    if len(logging.root.handlers) == 0:
        logging.basicConfig()
    root.warning(msg, *args, **kwargs)


def info(msg, *args, **kwargs):
    if len(logging.root.handlers) == 0:
        logging.basicConfig()
    root.info(msg, *args, **kwargs)


def debug(msg, *args, **kwargs):
    if len(logging.root.handlers) == 0:
        logging.basicConfig()
    root.debug(msg, *args, **kwargs)


def log(level, msg, *args, **kwargs):
    if len(logging.root.handlers) == 0:
        logging.basicConfig()
    root.log(level, msg, *args, **kwargs)
