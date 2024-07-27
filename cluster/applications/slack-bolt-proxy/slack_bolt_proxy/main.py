import asyncio
import collections.abc
import datetime
import hashlib
import hmac
import json
import logging
import re
import signal
import time
import typing

import aiohttp
import dotenv
import opentelemetry.context
import opentelemetry.exporter.otlp.proto.grpc.trace_exporter
import opentelemetry.exporter.prometheus
import opentelemetry.instrumentation.aiohttp_client
import opentelemetry.metrics
import opentelemetry.sdk.metrics
import opentelemetry.sdk.resources
import opentelemetry.sdk.trace.export
import opentelemetry.trace
import prometheus_client
import pythonjsonlogger.jsonlogger
import slack_bolt.adapter.socket_mode.async_handler
import slack_bolt.async_app
import slack_bolt.context.ack.async_ack
import slack_bolt.context.say.async_say
import slack_sdk.errors
import slack_sdk.socket_mode.aiohttp
import slack_sdk.socket_mode.request
import slack_sdk.web.async_client
import slack_sdk.web.async_slack_response

import slack_bolt_proxy.exceptions
import slack_bolt_proxy.settings
import slack_bolt_proxy.telemetry

s = slack_bolt_proxy.settings.Settings()
bolt = slack_bolt.async_app.AsyncApp(
    token=s.slack_bot_token,
    signing_secret=s.slack_signing_secret,
    process_before_response=s.slack_process_before_response,
)


async def startup():
    if s.load_dotenv:
        dotenv.load_dotenv(override=True)

    class JsonFormatter(pythonjsonlogger.jsonlogger.JsonFormatter):
        def add_fields(
            self,
            log_record: dict[str, typing.Any],
            record: logging.LogRecord,
            message_dict: dict[str, typing.Any],
        ):
            now = datetime.datetime.now()
            log_record["name"] = record.name
            log_record["time"] = now.isoformat()
            log_record["severitytext"] = record.levelname

            super().add_fields(log_record, record, message_dict)

    handler = logging.StreamHandler()
    handler.setFormatter(JsonFormatter())
    logging.basicConfig(level=s.convert_log_level(), handlers=[handler])

    provider = opentelemetry.sdk.trace.TracerProvider(
        resource=opentelemetry.sdk.resources.OTELResourceDetector().detect(),
    )
    processor = opentelemetry.sdk.trace.export.BatchSpanProcessor(
        opentelemetry.exporter.otlp.proto.grpc.trace_exporter.OTLPSpanExporter(),
    )
    provider.add_span_processor(processor)
    opentelemetry.trace.set_tracer_provider(provider)

    opentelemetry.instrumentation.aiohttp_client.AioHttpClientInstrumentor().instrument(
        url_filter=lambda url: str(url.with_query(None)),
    )

    opentelemetry.metrics.set_meter_provider(opentelemetry.sdk.metrics.MeterProvider(
        metric_readers=[opentelemetry.exporter.prometheus.PrometheusMetricReader()],
    ))


def signature(key: bytes, msg: bytes) -> str:
    return f"v0={hmac.new(key, msg, hashlib.sha256).hexdigest()}"


@bolt.event(re.compile(".*"))
@bolt.command(re.compile(".*"))
@bolt.shortcut(re.compile(".*"))
@bolt.action(re.compile(".*"))
@bolt.view(re.compile(".*"))
@bolt.options(re.compile(".*"))
async def handle(
    ack: slack_bolt.context.ack.async_ack,
    body: collections.abc.Mapping[str, typing.Any],
):
    await ack()
    async with aiohttp.ClientSession() as session:
        ts_ns = time.time()
        ts = int(ts_ns)
        body = json.dumps(body, ensure_ascii=False)
        async with session.post(
            f"http://{s.host}:{s.port}/slack/events",
            headers={
                "X-Slack-Request-Timestamp": str(ts),
                "X-Slack-Signature": signature(
                    s.slack_signing_secret.encode("utf-8"),
                    f"v0:{str(ts)}:{body}".encode("utf-8"),
                ),
            },
            data=body,
        ) as response:
            response.raise_for_status()


@bolt.error
async def custom_error_handler(error: Exception):
    if isinstance(error, slack_bolt_proxy.exceptions.RetryableError):
        slack_bolt_proxy.telemetry.logger.error(error)
    elif isinstance(error, Exception):
        slack_bolt_proxy.telemetry.logger.error(error, exc_info=error)


if __name__ == "__main__":
    prometheus_client.start_http_server(s.metrics_port)

    loop = asyncio.new_event_loop()

    loop.run_until_complete(startup())


    async def socket_mode_handler():
        class AsyncSocketModeHandlerWrapper(slack_bolt.adapter.socket_mode.async_handler.AsyncSocketModeHandler):
            async def handle(
                self,
                client: slack_sdk.socket_mode.aiohttp.SocketModeClient,
                req: slack_sdk.socket_mode.request.SocketModeRequest,
            ) -> None:
                event = req.payload.get("event")

                if event is None:
                    return await super().handle(client, req)

                with slack_bolt_proxy.telemetry.tracer.start_as_current_span(
                    "handle",
                    attributes={
                        "slack.team": event.get("team", event.get("user_team", "")),
                        "slack.user": event.get("user", event.get("username", "")),
                        "slack.channel": event["channel"],
                    } if event.get("team") else {
                        "slack.channel": event["channel"],
                    },
                ):
                    return await super().handle(client, req)

        # Timeout context manager should be used inside a task
        wrapper = AsyncSocketModeHandlerWrapper(
            bolt,
            s.slack_app_token,
        )

        task = loop.create_task(wrapper.start_async())

        async def graceful_shutdown():
            await wrapper.close_async()  # Not accepting new connections

            inflight_tasks = [
                t for t in asyncio.all_tasks()
                if t not in [root_task, task, asyncio.current_task()]
            ]

            await asyncio.gather(*inflight_tasks)

            task.cancel()

        loop.add_signal_handler(
            signal.SIGTERM,
            lambda: loop.create_task(graceful_shutdown()),
        )

        try:
            await task
        except asyncio.CancelledError:
            pass


    root_task = loop.create_task(socket_mode_handler())

    # compatible with asyncio.run()
    signal.signal(signal.SIGINT, lambda signum, frame: root_task.cancel())

    loop.run_until_complete(root_task)
