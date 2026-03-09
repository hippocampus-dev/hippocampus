import asyncio
import collections.abc
import contextlib
import datetime
import difflib
import logging
import re
import signal
import typing

import boto3
import fastapi
import opentelemetry.exporter.otlp.proto.grpc.trace_exporter
import opentelemetry.exporter.prometheus
import opentelemetry.instrumentation.aiohttp_client
import opentelemetry.instrumentation.botocore
import opentelemetry.instrumentation.fastapi
import opentelemetry.instrumentation.httpx
import opentelemetry.instrumentation.requests
import opentelemetry.metrics
import opentelemetry.sdk.metrics
import opentelemetry.sdk.resources
import opentelemetry.sdk.trace.export
import opentelemetry.trace
import prometheus_client
import pythonjsonlogger.jsonlogger
import slack_bolt.adapter.fastapi.async_handler
import slack_bolt.adapter.socket_mode.async_handler
import slack_bolt.async_app
import slack_bolt.context.say.async_say
import slack_sdk.errors
import slack_sdk.socket_mode.aiohttp
import slack_sdk.socket_mode.request
import slack_sdk.web.async_client
import slack_sdk.web.async_slack_response
import time

import slackug.brain
import slackug.context_logging
import slackug.exceptions
import slackug.settings
import slackug.slack.manager
import slackug.telemetry

s = slackug.settings.Settings()


async def startup():
    class JsonFormatter(pythonjsonlogger.jsonlogger.JsonFormatter):
        def __init__(self, *args, **kwargs):
            # https://opentelemetry.io/docs/specs/otel/logs/data-model/
            super().__init__(*args, **kwargs, rename_fields={"message": "body"})

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
    if s.is_debug():
        handler.setFormatter(logging.Formatter(
            "time=%(asctime)s name=%(name)s severitytext=%(levelname)s body=%(message)s traceid=%(traceid)s spanid=%(spanid)s",
            defaults={"traceid": "", "spanid": ""},
        )),
    logging.basicConfig(level=s.convert_log_level(), handlers=[handler])

    provider = opentelemetry.sdk.trace.TracerProvider(
        resource=opentelemetry.sdk.resources.OTELResourceDetector().detect(),
    )
    processor = opentelemetry.sdk.trace.export.BatchSpanProcessor(
        opentelemetry.exporter.otlp.proto.grpc.trace_exporter.OTLPSpanExporter(),
    )
    provider.add_span_processor(processor)
    opentelemetry.trace.set_tracer_provider(provider)

    opentelemetry.instrumentation.botocore.BotocoreInstrumentor().instrument()
    opentelemetry.instrumentation.aiohttp_client.AioHttpClientInstrumentor().instrument(
        url_filter=lambda url: str(url.with_query(None)),
    )
    opentelemetry.instrumentation.requests.RequestsInstrumentor().instrument()
    opentelemetry.instrumentation.httpx.HTTPXClientInstrumentor().instrument()

    opentelemetry.metrics.set_meter_provider(opentelemetry.sdk.metrics.MeterProvider(
        metric_readers=[opentelemetry.exporter.prometheus.PrometheusMetricReader()],
    ))

    with slackug.telemetry.tracer.start_as_current_span(
        "startup",
    ):
        match s.brain_type:
            case slackug.settings.BrainType.S3:
                s3_client = boto3.client("s3", endpoint_url=s.s3_endpoint_url)
                await slackug.slack.manager.register(bolt, slackug.brain.S3Brain(s3_client, s.s3_bucket))
            case _:
                raise NotImplementedError


async def shutdown():
    inflight_tasks = [
        t for t in asyncio.all_tasks()
        if t not in [asyncio.current_task()] and t.get_name() != "Task-1"
    ]
    await asyncio.gather(*inflight_tasks)


@contextlib.asynccontextmanager
async def lifespan(_app: fastapi.FastAPI):
    await startup()
    yield
    await shutdown()


bolt = slack_bolt.async_app.AsyncApp(
    token=s.slack_bot_token,
    signing_secret=s.slack_signing_secret,
    process_before_response=s.slack_process_before_response,
)
app = fastapi.FastAPI(lifespan=lifespan)
opentelemetry.instrumentation.fastapi.FastAPIInstrumentor.instrument_app(app)
app_handler = slack_bolt.adapter.fastapi.async_handler.AsyncSlackRequestHandler(bolt)

global_internal_errors_total: opentelemetry.metrics.Counter | None = None


def get_internal_errors_total() -> opentelemetry.metrics.Counter:
    global global_internal_errors_total
    if global_internal_errors_total is None:
        global_internal_errors_total = slackug.telemetry.meter.create_counter(
            "internal_errors_total",
            description="Total number of errors occurred",
        )
    return global_internal_errors_total


def validate_user(user: collections.abc.Mapping[str, typing.Any]) -> bool:
    if not s.allow_restricted_user:
        if user.get("is_restricted") or user.get("is_ultra_restricted"):
            return False

    if s.allow_teams:
        if user["team_id"] not in s.allow_teams:
            return False

    if s.allow_email_domains:
        email = user["profile"].get("email")
        if email is None:
            return False

        if not any(
            email.endswith(domain)
            for domain in s.allow_email_domains
        ):
            return False

    return True


user_cache: collections.abc.MutableMapping[str, typing.Any] = {}
channel_cache: collections.abc.MutableMapping[str, typing.Any] = {}


async def handle_event(
    retryable_client: slackug.slack.RetryableAsyncWebClient,
    event: collections.abc.Mapping[str, typing.Any],
    context: slackug.slack.SlackContext,
):
    text = event.get("text", "").strip()
    if text == "":
        return

    ug_match = re.search(r'(?:^|\s)!ug\s+(\S+)', text)
    if not ug_match:
        return

    brain: slackug.brain.Brain | None = None
    match s.brain_type:
        case slackug.settings.BrainType.S3:
            brain = slackug.brain.S3Brain(boto3.client("s3", endpoint_url=s.s3_endpoint_url), s.s3_bucket)
        case _:
            raise NotImplementedError

    if brain is None:
        return

    restored_groups = await brain.restore("usergroups")
    groups = slackug.slack.manager.UserGroups.model_validate_json(
        restored_groups
    ) if restored_groups else slackug.slack.manager.UserGroups.model_validate({})

    ug = ug_match.group(1)
    group = groups.get(ug)

    if group is not None and (group.channel_id is None or group.channel_id == event["channel"]):
        users_text = " ".join([f"<@{user}>" for user in group.users])
        await retryable_client.chat_postMessage(
            channel=event["channel"],
            thread_ts=event.get("thread_ts") or event["ts"],
            text=users_text,
        )
    else:
        scoped_groups = []
        for group_name in groups.keys():
            group = groups[group_name]
            if group.channel_id is None or group.channel_id == event["channel"]:
                scoped_groups.append(group_name)

        if scoped_groups:
            matches = difflib.get_close_matches(ug, scoped_groups, n=3, cutoff=0.6)
            if matches:
                await retryable_client.chat_postEphemeral(
                    channel=event["channel"],
                    thread_ts=event.get("thread_ts") or event["ts"],
                    user=event.get("user", event.get("username")),
                    text="Did you mean?: " + ", ".join([f"`{match}`" for match in matches]),
                )


@bolt.event("message")
async def handle_message(
    say: slack_bolt.context.say.async_say.AsyncSay,
    client: slack_sdk.web.async_client.AsyncWebClient,
    body: collections.abc.Mapping[str, typing.Any],
):
    event: collections.abc.Mapping[str, typing.Any] = body["event"]

    if event.get("user", event.get("username")) == s.slack_bot_member_id:
        return

    if s.slack_process_before_response:
        if time.time() - float(event["event_ts"]) > 3:  # Deduplicate
            return

    with (slackug.telemetry.tracer.start_as_current_span(
        "handle_message",
        attributes={
            "slack.team": event.get("team", event.get("user_team", "")),
            "slack.user": event.get("user", event.get("username", "")),
            "slack.channel": event["channel"],
            "slack.thread": event.get("thread_ts") or event["ts"],
        },
    )):
        retryable_client = slackug.slack.RetryableAsyncWebClient(client)

        context = slackug.slack.SlackContext(user={})

        if channel_cache.get(event["channel"]) is None:
            channel_info = await retryable_client.conversations_info(  # Tier 3 (50+ per minute)
                channel=event["channel"],
            )
            channel_cache[event["channel"]] = channel_info["channel"]

        context.channel = channel_cache[event["channel"]]

        if (
            event.get("subtype") not in ["bot_message"]
        ) and (
            user := event.get("user", event.get("username"))
        ) is not None:
            if user_cache.get(user) is None:
                user_info = await retryable_client.users_info(  # Tier 4 (100+ per minute)
                    user=user,
                )
                user_cache[user] = user_info["user"]

            context.user = user_cache[user]

            if not s.allow_restricted_user or s.allow_teams or s.allow_email_domains:
                if not validate_user(context.user):
                    await say(
                        slackug.slack.i18n.translate(
                            "You are not available.",
                            locale=context.locale,
                        ),
                        thread_ts=event.get("thread_ts") or event["ts"],
                    )
                    return

        if not s.allow_ext_shared_channel:
            if body["is_ext_shared_channel"]:
                await say(
                    slackug.slack.i18n.translate(
                        "Not available on this channel.",
                        locale=context.locale,
                    ),
                    thread_ts=event.get("thread_ts") or event["ts"],
                )
                return

        if s.allow_channels:
            if event["channel"] not in s.allow_channels:
                await say(
                    slackug.slack.i18n.translate(
                        "Not available on this channel.",
                        locale=context.locale,
                    ),
                    thread_ts=event.get("thread_ts") or event["ts"],
                )
                return

        await handle_event(retryable_client, event, context)


@bolt.error
async def custom_error_handler(error: Exception):
    if isinstance(error, slackug.exceptions.RetryableError):
        get_internal_errors_total().add(1, attributes={"retryable": "true"})
        slackug.telemetry.logger.error(error)
    elif isinstance(error, Exception):
        get_internal_errors_total().add(1, attributes={"retryable": "false"})
        slackug.telemetry.logger.error(error, exc_info=error)


@bolt.event("member_joined_channel")
async def handle_member_joined_channel_events(
    say: slack_bolt.context.say.async_say.AsyncSay,
    client: slack_sdk.web.async_client.AsyncWebClient,
    body: collections.abc.Mapping[str, typing.Any],
):
    event: collections.abc.Mapping[str, typing.Any] = body["event"]

    if event.get("user", event.get("username")) == client.auth_test().get("user_id"):
        say("Hi, I'm Translator. You can configure me by `/translation`.")


@app.post("/slack/events")
async def endpoint(req: fastapi.Request):
    return await app_handler.handle(req)


@app.get("/healthz")
def health() -> str:
    return "OK"


@app.get("/metrics")
def metrics():
    return fastapi.Response(
        prometheus_client.generate_latest(),
        media_type=prometheus_client.CONTENT_TYPE_LATEST,
    )


if __name__ == "__main__":
    if s.is_debug():
        import dotenv

        dotenv.load_dotenv(override=True)

    if s.slack_app_token:
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

                    with slackug.telemetry.tracer.start_as_current_span(
                        "handle",
                        attributes={
                            "slack.team": event["team"],
                            "slack.user": event.get("user", event.get("username", "")),
                            "slack.channel": event["channel"],
                        } if "team" in event else {
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
    else:
        import uvicorn

        uvicorn.run(
            app,
            host=s.host,
            port=s.port,
            log_level=s.convert_log_level(),
            timeout_keep_alive=s.idle_timeout,
            timeout_graceful_shutdown=s.termination_grace_period_seconds,
            access_log=False,
        )
