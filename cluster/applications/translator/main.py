import asyncio
import collections.abc
import contextlib
import datetime
import json
import logging
import os
import re
import signal
import typing

import boto3
import fastapi
import httpx
import openai
import opentelemetry.exporter.otlp.proto.grpc.trace_exporter
import opentelemetry.exporter.prometheus
import opentelemetry.instrumentation.aiohttp_client
import opentelemetry.instrumentation.botocore
import opentelemetry.instrumentation.fastapi
import opentelemetry.instrumentation.httpx
import opentelemetry.instrumentation.redis
import opentelemetry.instrumentation.requests
import opentelemetry.metrics
import opentelemetry.sdk.metrics
import opentelemetry.sdk.resources
import opentelemetry.sdk.trace.export
import opentelemetry.trace
import prometheus_client
import pydantic
import pythonjsonlogger.jsonlogger
import redis.asyncio
import redis.asyncio.retry
import redis.backoff
import redis.exceptions
import slack_bolt.adapter.fastapi.async_handler
import slack_bolt.adapter.socket_mode.async_handler
import slack_bolt.async_app
import slack_bolt.context.say.async_say
import slack_sdk.errors
import slack_sdk.socket_mode.aiohttp
import slack_sdk.socket_mode.request
import slack_sdk.web.async_client
import slack_sdk.web.async_slack_response
import tiktoken
import time

import cortex.brain
import cortex.exceptions
import cortex.factory
import cortex.llm.openai.agent.memory
import cortex.llm.openai.model
import cortex.rate_limit
import cortex.throttled_function
import translator.context_logging
import translator.settings
import translator.slack.customize
import translator.slack.expand
import translator.telemetry

s = translator.settings.Settings()

try:
    encoder: tiktoken.Encoding = tiktoken.encoding_for_model(s.model)
except KeyError:
    encoder: tiktoken.Encoding = tiktoken.get_encoding("cl100k_base")


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
    opentelemetry.instrumentation.redis.RedisInstrumentor().instrument()

    opentelemetry.metrics.set_meter_provider(opentelemetry.sdk.metrics.MeterProvider(
        metric_readers=[opentelemetry.exporter.prometheus.PrometheusMetricReader()],
    ))

    with translator.telemetry.tracer.start_as_current_span(
        "startup",
    ):
        match s.brain_type:
            case translator.settings.BrainType.S3:
                s3_client = boto3.client("s3", endpoint_url=s.s3_endpoint_url)
                await translator.slack.customize.register(bolt, cortex.brain.S3Brain(s3_client, s.s3_bucket))
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
        global_internal_errors_total = translator.telemetry.meter.create_counter(
            "internal_errors_total",
            description="Total number of errors occurred",
        )
    return global_internal_errors_total


global_processed_tokens_total: opentelemetry.metrics.Counter | None = None


def get_processed_tokens_total() -> opentelemetry.metrics.Counter:
    global global_processed_tokens_total
    if global_processed_tokens_total is None:
        global_processed_tokens_total = translator.telemetry.meter.create_counter(
            "processed_tokens_total",
            description="Total number of tokens processed",
        )
    return global_processed_tokens_total


global_rate_limiter: cortex.rate_limit.RateLimiter | None = None


async def get_rate_limiter() -> cortex.rate_limit.RateLimiter:
    global global_rate_limiter
    if global_rate_limiter is None:
        match s.rate_limiter_type:
            case cortex.llm.openai.agent.MemoryType.Redis:
                redis_client = redis.asyncio.Redis(
                    host=s.redis_host,
                    port=s.redis_port,
                    retry=redis.asyncio.retry.Retry(redis.backoff.ExponentialBackoff(), 3),
                    retry_on_error=[redis.exceptions.ConnectionError, redis.exceptions.ReadOnlyError],
                )
                # HACK: execute_command ignores auto_close_connection_pool
                import types

                async def new_execute_command(self, *args, **options):
                    try:
                        return await redis.asyncio.Redis.execute_command(self, *args, **options)
                    except (redis.exceptions.ConnectionError, redis.exceptions.ReadOnlyError) as e:
                        raise cortex.exceptions.RetryableError(e)
                    finally:
                        if self.auto_close_connection_pool:
                            await self.connection_pool.disconnect()

                redis_client.execute_command = types.MethodType(new_execute_command, redis_client)
                global_rate_limiter = cortex.rate_limit.RedisSlidingRateLimiter(
                    redis_client,
                    interval_seconds=s.rate_limit_interval_seconds,
                )
            case _:
                raise NotImplementedError
    return global_rate_limiter


def calculate_model_price(
    prompt_tokens: collections.abc.Mapping[cortex.llm.openai.model.CompletionModel, int],
    completion_tokens: collections.abc.Mapping[cortex.llm.openai.model.CompletionModel, int],
) -> float:
    price = 0.0
    for model, tokens in prompt_tokens.items():
        price += tokens * model.prices["price_per_prompt"]
    for model, tokens in completion_tokens.items():
        price += tokens * model.prices["price_per_completion"]
    return round(price, 6)


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


def create_channel_scoped_api_client_factory(
    token: str,
) -> typing.Callable[[str], typing.Awaitable[slack_sdk.web.async_client.AsyncWebClient | None]]:
    conversations_join_cache: collections.abc.MutableMapping[str, bool] = {}

    async def factory(channel: str) -> slack_sdk.web.async_client.AsyncWebClient | None:
        client = slack_sdk.web.async_client.AsyncWebClient(
            token=token,
        )

        if conversations_join_cache.get(channel) is None:
            try:
                await client.conversations_join(channel=channel)
            except slack_sdk.errors.SlackApiError as e:
                if e.response["error"] == "channel_not_found":
                    conversations_join_cache[channel] = False
                else:
                    raise e
            else:
                conversations_join_cache[channel] = True

        if not conversations_join_cache[channel]:
            return None

        return client

    return factory


api_client_factory = cortex.factory.RoundRobinFactory(
    [
        create_channel_scoped_api_client_factory(token)
        for token in s.slack_api_client_tokens
    ],
)


class Result(pydantic.BaseModel):
    language: translator.slack.customize.Locale
    text: str


class ResponseFormat(pydantic.BaseModel):
    original_language: translator.slack.customize.Locale
    results: collections.abc.Sequence[Result]


EMOJI_REGEXP = re.compile(r"^:[^:\s]+:$")


async def handle_event(
    upsert: typing.Callable[..., typing.Awaitable[slack_sdk.web.async_slack_response.AsyncSlackResponse]],
    event: collections.abc.Mapping[str, typing.Any],
    context: translator.slack.SlackContext,
):
    text = event.get("text", "").strip()
    if text == "":
        return

    if EMOJI_REGEXP.fullmatch(text):
        return

    rate_limiter = await get_rate_limiter()
    rate_limiter_key = event["channel"]
    if not await rate_limiter.remaining(rate_limiter_key, limit=context.limit or s.rate_limit_per_interval):
        await upsert(
            channel=event["channel"],
            thread_ts=event.get("thread_ts") or event["ts"],
            text=translator.slack.i18n.translate(
                "You have reached {rate_limit_interval_seconds}-second usage limit. Please wait a moment and try again.",
                locale=context.locale,
            ).format(
                rate_limit_interval_seconds=s.rate_limit_interval_seconds,
            )
        )
        return

    brain: cortex.brain.Brain | None = None
    match s.brain_type:
        case translator.settings.BrainType.S3:
            brain = cortex.brain.S3Brain(boto3.client("s3", endpoint_url=s.s3_endpoint_url), s.s3_bucket)
        case _:
            raise NotImplementedError

    translation = translator.slack.customize.Translation()
    if brain is not None:
        restored_translations = await brain.restore("translations")
        translations = (
            translator.slack.customize.Translations.model_validate_json(restored_translations)
            if restored_translations is not None
            else translator.slack.customize.Translations.model_validate({})
        )
        if event["channel"] in translations:
            translation = translations[event["channel"]]

    if translation.enabled == translator.slack.customize.Toggle.Off:
        return

    system_prompt = "Translate the given text according to the following rules."
    system_prompt += "\n- **NEVER change the document structure each languages, including line breaks, URLs, code snippets, etc**"
    system_prompt += "\n- **DONT include the original language in the result**"
    for i, locale in enumerate(translation.locales):
        others = [other for j, other in enumerate(translation.locales) if i != j]
        system_prompt += f"\n- Translate {locale} to {', '.join(others)}"
    messages = [{
        "role": "system",
        "content": system_prompt,
    }, {
        "role": "user",
        "content": text,
    }]

    try:
        try:
            response = await cortex.llm.openai.AsyncOpenAI(
                http_client=httpx.AsyncClient(timeout=None, mounts={
                    "http://": httpx.AsyncHTTPTransport(proxy=os.getenv("HTTP_PROXY")),
                    "https://": httpx.AsyncHTTPTransport(proxy=os.getenv("HTTPS_PROXY")),
                }, verify=os.getenv("SSL_CERT_FILE")),
            ).beta.chat.completions.parse(
                model=s.model.replace(".", "") if os.getenv("OPENAI_API_TYPE") == "azure" else s.model,
                messages=messages,
                max_tokens=s.model.max_completion_tokens,
                response_format=ResponseFormat,
            )
        except openai.RateLimitError:
            await upsert(
                channel=event["channel"],
                thread_ts=event.get("thread_ts") or event["ts"],
                text=translator.slack.i18n.translate(
                    "You have reached your OpenAI API billing limit.\nPlease wait a moment and try again.",
                    locale=context.locale,
                ),
            )
            return
        except openai.BadRequestError as e:
            match e.code:
                case "billing_hard_limit_reached":
                    await upsert(
                        channel=event["channel"],
                        thread_ts=event.get("thread_ts") or event["ts"],
                        text=translator.slack.i18n.translate(
                            "You have reached your OpenAI API billing limit.\nPlease wait a moment and try again.",
                            locale=context.locale,
                        ),
                    )
                    return
                case "context_length_exceeded":
                    await upsert(
                        channel=event["channel"],
                        thread_ts=event.get("thread_ts") or event["ts"],
                        text=translator.slack.i18n.translate(
                            "The maximum number of conversations has been exceeded.",
                            locale=context.locale,
                        ),
                    )
                    return
                case "content_policy_violation":
                    await upsert(
                        channel=event["channel"],
                        thread_ts=event.get("thread_ts") or event["ts"],
                        text=translator.slack.i18n.translate(
                            "Your message contains violent or explicit.",
                            locale=context.locale,
                        ),
                    )
                    return
            raise e
        except openai.APIConnectionError as e:
            raise cortex.exceptions.RetryableError(e) from e
        except openai.APIStatusError as e:
            match e.status_code:
                case 404:
                    if e.message == "Engine not found":
                        raise cortex.exceptions.RetryableError(e) from e
                case 409 | 429 | 502 | 503 | 504:
                    raise cortex.exceptions.RetryableError(e) from e
            raise e
        except openai.APIError as e:
            match e.code:
                case "server_error" | "rate_limit_exceeded":
                    raise cortex.exceptions.RetryableError(e) from e
            raise e

        tasks = []

        choice = response.choices[0]
        match choice.finish_reason:
            case "length":
                await upsert(
                    channel=event["channel"],
                    thread_ts=event.get("thread_ts") or event["ts"],
                    text=translator.slack.i18n.translate(
                        "The maximum number of conversations has been exceeded.",
                        locale=context.locale,
                    ),
                )
                return
            case "content_filter":
                await upsert(
                    channel=event["channel"],
                    thread_ts=event.get("thread_ts") or event["ts"],
                    text=translator.slack.i18n.translate(
                        "Your message contains violent or explicit.",
                        locale=context.locale,
                    ),
                )
                return
            case "tool_calls" | "stop" | None:
                pass
            case _ as finish_reason:
                raise ValueError(f"Unknown finish_reason: {finish_reason}")

        response_format = ResponseFormat.model_validate_json(choice.message.content)
        response_format.results = sorted(
            list(
                {
                    result.language: result for result in response_format.results
                    if result.language in translation.locales
                }.values()
            ),
            key=lambda result: translation.locales.index(result.language),
        )

        blocks = []
        actions = {
            "type": "actions",
            "elements": [],
        }
        await translator.slack.expand.register(bolt, brain)
        for i, result in enumerate(response_format.results):
            transformed_message = translator.slack.transform_message(
                result.text.replace("```", "\n```\n"),
                translator.slack.SlackMarkdownRenderer(
                    url_shortener=cortex.URLShortener(s.url_shortener_url),
                ),
            )

            # `section` element does not support `text` longer than 3000 characters
            if translation.collapse == translator.slack.customize.Toggle.On or len(transformed_message) > 3000:
                key = f"expand/{event['channel']}/{event['ts']}/{i}"
                # `button` element does not support `text` longer than 2000 characters
                await brain.save(
                    key,
                    transformed_message.encode("utf-8"),
                )
                actions["elements"].append({
                    "type": "button",
                    "text": {
                        "type": "plain_text",
                        "text": result.language,
                    },
                    "action_id": f"expand_{i}",
                    "value": key,
                })
            else:
                if len(response_format.results) > 1:
                    blocks.append({
                        "type": "header",
                        "text": {
                            "type": "plain_text",
                            "text": result.language,
                        },
                    })
                blocks.append({
                    "type": "section",
                    "text": {
                        "type": "mrkdwn",
                        "text": transformed_message,
                    },
                })

        if len(actions["elements"]) > 0:
            blocks.append(actions)

        async def _task():
            await upsert(
                channel=event["channel"],
                thread_ts=event.get("thread_ts") or event["ts"],
                text=text,
                blocks=blocks,
            )

        tasks.append(_task())

        tasks.append(
            rate_limiter.take(rate_limiter_key, response.usage.prompt_tokens + response.usage.completion_tokens)
        )

        await asyncio.gather(*tasks)
    except cortex.exceptions.RetryableError as e:
        await upsert(
            channel=event["channel"],
            thread_ts=event.get("thread_ts") or event["ts"],
            text=translator.slack.i18n.translate(
                "A temporary error has occurred.\nPlease try again.",
                locale=context.locale,
            ),
        )
        raise e
    except Exception as e:
        await upsert(
            channel=event["channel"],
            thread_ts=event.get("thread_ts") or event["ts"],
            text=translator.slack.i18n.translate(
                "An unknown error has occurred",
                locale=context.locale,
            ),
        )
        raise e

    translator.telemetry.logger.info(
        json.dumps({
            "team": event.get("team", event.get("user_team", "")),
            "user": event.get("user", event.get("username", "")),
            "channel": event["channel"],
            "thread": event.get("thread_ts") or event["ts"],
            "prompt_tokens": response.usage.prompt_tokens,
            "completion_tokens": response.usage.completion_tokens,
        }),
    )

    get_processed_tokens_total().add(
        response.usage.prompt_tokens,
        attributes={
            "usage": "prompt",
            "model": s.model,
            "slack.team": event.get("team", event.get("user_team", "")),
            "slack.user": event.get("user", event.get("username", "")),
            "slack.channel": event["channel"],
        },
    )
    get_processed_tokens_total().add(
        response.usage.completion_tokens,
        attributes={
            "usage": "completion",
            "model": s.model,
            "slack.team": event.get("team", event.get("user_team", "")),
            "slack.user": event.get("user", event.get("username", "")),
            "slack.channel": event["channel"],
        },
    )


async def get_previous_message(
    client: slack_sdk.web.async_client.AsyncWebClient,
    event: collections.abc.Mapping[str, typing.Any],
) -> collections.abc.Mapping[str, typing.Any] | None:
    previous_message = event.get("previous_message")
    if previous_message is None:
        return None

    response = await client.conversations_replies(
        channel=event["channel"],
        ts=previous_message.get("thread_ts") or previous_message["ts"],
        oldest=previous_message["ts"],
    )
    previous_message_text = previous_message["text"].replace("\n", " ")
    for message in response["messages"]:
        user = message.get("user", event.get("username"))
        if user == s.slack_bot_member_id and message["text"] == previous_message_text:
            return message

    return None


@bolt.event("message")
async def handle_message(
    say: slack_bolt.context.say.async_say.AsyncSay,
    client: slack_sdk.web.async_client.AsyncWebClient,
    body: collections.abc.Mapping[str, typing.Any],
):
    event: collections.abc.Mapping[str, typing.Any] = body["event"]

    if "bot_id" in event:
        return

    if event.get("user", event.get("username")) == s.slack_bot_member_id:
        return

    if s.slack_process_before_response:
        if time.time() - float(event["event_ts"]) > 3:  # Deduplicate
            return

    with translator.telemetry.tracer.start_as_current_span(
        "handle_message",
        attributes={
            "model": s.model,
            "slack.team": event.get("team", event.get("user_team", "")),
            "slack.user": event.get("user", event.get("username", "")),
            "slack.channel": event["channel"],
            "slack.thread": event.get("thread_ts") or event["ts"],
        },
    ):
        if event["channel_type"] != "im":
            api_client = await api_client_factory.construct(event["channel"])
            if api_client is not None:
                client = api_client

        retryable_client = translator.slack.RetryableAsyncWebClient(client)
        upsert = retryable_client.chat_postMessage  # Special Tier (1 per second per channel)

        if "subtype" in event:
            if event["subtype"] == "message_deleted":
                deleted_message = await get_previous_message(retryable_client, event)
                if deleted_message is None:
                    return

                await retryable_client.chat_delete(
                    channel=event["channel"],
                    ts=deleted_message["ts"],
                )
                return
            elif event["subtype"] == "message_changed":
                changed_message = await get_previous_message(retryable_client, event)
                if changed_message is None:
                    return

                async def chat_update_changed_message(**kwargs):
                    await retryable_client.chat_update(  # Tier 3 (50+ per minute)
                        ts=changed_message["ts"],
                        **kwargs,
                    )

                upsert = chat_update_changed_message
                event: dict = {**event, **event["message"]}
                del event["message"]
                del event["previous_message"]
            elif event["subtype"] not in ["file_share", "thread_broadcast"]:
                return

        context = translator.slack.SlackContext(user={})

        if channel_cache.get(event["channel"]) is None:
            channel_info = await retryable_client.conversations_info(  # Tier 3 (50+ per minute)
                channel=event["channel"],
            )
            channel_cache[event["channel"]] = channel_info["channel"]

        context.channel = channel_cache[event["channel"]]

        if (user := event.get("user", event.get("username"))) is not None:
            if user_cache.get(user) is None:
                user_info = await retryable_client.users_info(  # Tier 4 (100+ per minute)
                    user=user,
                )
                user_cache[user] = user_info["user"]

            context.user = user_cache[user]

            print(context.user)
            if not s.allow_restricted_user or s.allow_teams or s.allow_email_domains:
                if not validate_user(context.user):
                    await say(
                        translator.slack.i18n.translate(
                            "You are not available.",
                            locale=context.locale,
                        ),
                        thread_ts=event.get("thread_ts") or event["ts"],
                    )
                    return

        if not s.allow_ext_shared_channel:
            if body["is_ext_shared_channel"]:
                await say(
                    translator.slack.i18n.translate(
                        "Not available on this channel.",
                        locale=context.locale,
                    ),
                    thread_ts=event.get("thread_ts") or event["ts"],
                )
                return

        if s.allow_channels:
            if event["channel"] not in s.allow_channels:
                await say(
                    translator.slack.i18n.translate(
                        "Not available on this channel.",
                        locale=context.locale,
                    ),
                    thread_ts=event.get("thread_ts") or event["ts"],
                )
                return

        await handle_event(upsert, event, context)


@bolt.error
async def custom_error_handler(error: Exception):
    if isinstance(error, cortex.exceptions.RetryableError):
        get_internal_errors_total().add(1, attributes={"retryable": "true"})
        translator.telemetry.logger.error(error)
    elif isinstance(error, Exception):
        get_internal_errors_total().add(1, attributes={"retryable": "false"})
        translator.telemetry.logger.error(error, exc_info=error)


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

                    with translator.telemetry.tracer.start_as_current_span(
                        "handle",
                        attributes={
                            "model": s.model,
                            "slack.team": event["team"],
                            "slack.user": event.get("user", event.get("username", "")),
                            "slack.channel": event["channel"],
                        } if "team" in event else {
                            "model": s.model,
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
