import asyncio
import base64
import collections.abc
import contextlib
import copy
import datetime
import json
import logging
import os
import random
import signal
import time
import typing

import aiohttp
import aiohttp.client_exceptions
import aredis_om
import boto3
import fastapi
import google.oauth2.credentials
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
import playwright
import playwright.async_api
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

import bot.agent.root_agent
import bot.context_logging
import bot.settings
import bot.slack.context_manager
import bot.slack.customize
import bot.slack.reporter
import bot.telemetry
import cortex.brain
import cortex.exceptions
import cortex.factory
import cortex.llm.openai.agent.memory
import cortex.llm.openai.model
import cortex.rate_limit
import cortex.throttled_function

s = bot.settings.Settings()

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
        (
            handler.setFormatter(
                logging.Formatter(
                    "time=%(asctime)s name=%(name)s severitytext=%(levelname)s body=%(message)s traceid=%(traceid)s spanid=%(spanid)s",
                    defaults={"traceid": "", "spanid": ""},
                )
            ),
        )
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

    opentelemetry.metrics.set_meter_provider(
        opentelemetry.sdk.metrics.MeterProvider(
            metric_readers=[opentelemetry.exporter.prometheus.PrometheusMetricReader()],
        )
    )

    with bot.telemetry.tracer.start_as_current_span(
        "startup",
    ):
        match s.memory_type:
            case cortex.llm.openai.agent.MemoryType.Redis:
                redis_client = redis.asyncio.Redis(
                    host=s.redis_host,
                    port=s.redis_port,
                    retry=redis.asyncio.retry.Retry(
                        redis.backoff.ExponentialBackoff(), 3
                    ),
                    retry_on_error=[
                        redis.exceptions.ConnectionError,
                        redis.exceptions.ReadOnlyError,
                    ],
                )
                # HACK: execute_command ignores auto_close_connection_pool
                import types

                async def new_execute_command(self, *args, **options):
                    try:
                        return await redis.asyncio.Redis.execute_command(
                            self, *args, **options
                        )
                    except (
                            redis.exceptions.ConnectionError,
                            redis.exceptions.ReadOnlyError,
                    ) as e:
                        raise cortex.exceptions.RetryableError(e)
                    finally:
                        if self.auto_close_connection_pool:
                            await self.connection_pool.disconnect()

                redis_client.execute_command = types.MethodType(
                    new_execute_command, redis_client
                )
                cortex.llm.openai.agent.memory.RedisMemory.Meta.database = redis_client
                await aredis_om.Migrator().run()

        match s.brain_type:
            case bot.settings.BrainType.S3:
                s3_client = boto3.client("s3", endpoint_url=s.s3_endpoint_url)
                await bot.slack.context_manager.register(
                    bolt, cortex.brain.S3Brain(s3_client, s.s3_bucket)
                )
                await bot.slack.customize.register(
                    bolt, cortex.brain.S3Brain(s3_client, s.s3_bucket)
                )
                await bot.slack.reporter.register(
                    bolt, cortex.brain.S3Brain(s3_client, s.s3_bucket)
                )
            case _:
                raise NotImplementedError


async def shutdown():
    inflight_tasks = [
        t
        for t in asyncio.all_tasks()
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
        global_internal_errors_total = bot.telemetry.meter.create_counter(
            "internal_errors_total",
            description="Total number of errors occurred",
        )
    return global_internal_errors_total


global_processed_tokens_total: opentelemetry.metrics.Counter | None = None


def get_processed_tokens_total() -> opentelemetry.metrics.Counter:
    global global_processed_tokens_total
    if global_processed_tokens_total is None:
        global_processed_tokens_total = bot.telemetry.meter.create_counter(
            "processed_tokens_total",
            description="Total number of tokens processed",
        )
    return global_processed_tokens_total


global_generated_images_total: opentelemetry.metrics.Counter | None = None


def get_generated_images_total() -> opentelemetry.metrics.Counter:
    global global_generated_images_total
    if global_generated_images_total is None:
        global_generated_images_total = bot.telemetry.meter.create_counter(
            "generated_images_total",
            description="Total number of images generated",
        )
    return global_generated_images_total


global_rate_limiter: cortex.rate_limit.RateLimiter | None = None


async def get_rate_limiter() -> cortex.rate_limit.RateLimiter:
    global global_rate_limiter
    if global_rate_limiter is None:
        match s.rate_limiter_type:
            case cortex.llm.openai.agent.MemoryType.Redis:
                redis_client = redis.asyncio.Redis(
                    host=s.redis_host,
                    port=s.redis_port,
                    retry=redis.asyncio.retry.Retry(
                        redis.backoff.ExponentialBackoff(), 3
                    ),
                    retry_on_error=[
                        redis.exceptions.ConnectionError,
                        redis.exceptions.ReadOnlyError,
                    ],
                )
                # HACK: execute_command ignores auto_close_connection_pool
                import types

                async def new_execute_command(self, *args, **options):
                    try:
                        return await redis.asyncio.Redis.execute_command(
                            self, *args, **options
                        )
                    except (
                            redis.exceptions.ConnectionError,
                            redis.exceptions.ReadOnlyError,
                    ) as e:
                        raise cortex.exceptions.RetryableError(e)
                    finally:
                        if self.auto_close_connection_pool:
                            await self.connection_pool.disconnect()

                redis_client.execute_command = types.MethodType(
                    new_execute_command, redis_client
                )
                global_rate_limiter = cortex.rate_limit.RedisSlidingRateLimiter(
                    redis_client,
                    interval_seconds=s.rate_limit_interval_seconds,
                )
            case _:
                raise NotImplementedError
    return global_rate_limiter


def calculate_model_price(
    prompt_tokens: collections.abc.Mapping[
        cortex.llm.openai.model.CompletionModel, int
    ],
    completion_tokens: collections.abc.Mapping[
        cortex.llm.openai.model.CompletionModel, int
    ],
    embedding_tokens: collections.abc.Mapping[
        cortex.llm.openai.model.EmbeddingModel, int
    ],
    generated_images: collections.abc.Mapping[cortex.llm.openai.model.ImageModel, int],
    processed_audio_seconds: collections.abc.Mapping[
        cortex.llm.openai.model.AudioModel, float
    ],
    converted_text_characters: collections.abc.Mapping[
        cortex.llm.openai.model.AudioModel, int
    ],
) -> float:
    price = 0.0
    for model, tokens in prompt_tokens.items():
        price += tokens * model.prices["price_per_prompt"]
    for model, tokens in completion_tokens.items():
        price += tokens * model.prices["price_per_completion"]
    for model, tokens in embedding_tokens.items():
        price += tokens * model.price
    for model, images in generated_images.items():
        price += images * model.price
    for model, seconds in processed_audio_seconds.items():
        price += seconds * model.price
    for model, characters in converted_text_characters.items():
        price += characters * model.price
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

        if not any(email.endswith(domain) for domain in s.allow_email_domains):
            return False

    return True


user_cache: collections.abc.MutableMapping[str, typing.Any] = {}
channel_cache: collections.abc.MutableMapping[str, typing.Any] = {}


def create_channel_scoped_api_client_factory(
    token: str,
) -> typing.Callable[
    [str], typing.Awaitable[slack_sdk.web.async_client.AsyncWebClient | None]
]:
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


api_client_factory = cortex.factory.ConsistentHashFactory(
    [
        create_channel_scoped_api_client_factory(token)
        for token in s.slack_api_client_tokens
    ],
)


async def handle_event_with_retry(
    client: slack_sdk.web.async_client.AsyncWebClient,
    retryable_client: bot.slack.RetryableAsyncWebClient,
    event: collections.abc.Mapping[str, typing.Any],
    context: bot.slack.SlackContext,
):
    i = 0

    async def retry(attempts: int, next_delay: int, max_delay: int | None = None):
        nonlocal i

        if max_delay is None:
            delay = next_delay
        else:
            delay = min(next_delay, max_delay)

        try:
            await handle_event(client, retryable_client, event, context)
        except cortex.exceptions.RetryableError as e:
            if i >= attempts:
                raise cortex.exceptions.RetryableError(e) from e

            await asyncio.sleep(delay * random.random())

            if next_delay <= (2 ** 63 - 1) / 2:
                next_delay *= 2
            else:
                next_delay = 2 ** 63 - 1

            i += 1
            return await retry(attempts, next_delay, max_delay)

    return await retry(attempts=3, next_delay=1, max_delay=10)


async def handle_event(
    client: slack_sdk.web.async_client.AsyncWebClient,
    retryable_client: bot.slack.RetryableAsyncWebClient,
    event: collections.abc.Mapping[str, typing.Any],
    context: bot.slack.SlackContext,
):
    rate_limiter = await get_rate_limiter()
    rate_limiter_key = event.get("user", event.get("username", ""))
    if not await rate_limiter.remaining(
        rate_limiter_key, limit=context.limit or s.rate_limit_per_interval
    ):
        await retryable_client.chat_postMessage(  # Special Tier (1 per second per channel)
            channel=event["channel"],
            thread_ts=event.get("thread_ts") or event["ts"],
            text=bot.slack.i18n.translate(
                "You have reached {rate_limit_interval_seconds}-second usage limit. Please wait a moment and try again.",
                locale=context.locale,
            ).format(
                rate_limit_interval_seconds=s.rate_limit_interval_seconds,
            ),
        )
        return

    replies = await retryable_client.conversations_replies(  # Tier 3 (50+ per minute)
        channel=event["channel"],
        ts=event.get("thread_ts") or event["ts"],
    )

    if replies["messages"] is None:
        return

    instructions = []

    if s.system_prompt is not None:
        instructions.append(s.system_prompt)

    brain: cortex.brain.Brain | None = None
    match s.brain_type:
        case bot.settings.BrainType.S3:
            brain = cortex.brain.S3Brain(
                boto3.client("s3", endpoint_url=s.s3_endpoint_url), s.s3_bucket
            )
        case _:
            raise NotImplementedError

    if brain is None:
        return

    avatar: bot.slack.customize.Avatar | None = None
    restored_avatars = await brain.restore("avatars")
    try:
        avatars = (
            bot.slack.customize.Avatars.model_validate_json(restored_avatars)
            if restored_avatars is not None
            else bot.slack.customize.Avatars.model_validate({})
        )
        avatar = avatars.get(event["channel"])
    except pydantic.ValidationError:
        pass

    model = s.model
    reasoning_effort = "medium"
    verbosity = "medium"
    if avatar is not None:
        reasoning_effort = avatar.reasoning_effort
        verbosity = avatar.verbosity
        model = avatar.model
        context.locale = avatar.locale
        if avatar.enabled == bot.slack.customize.Toggle.Off:
            avatar = None

    instructions.append(
        "Your task is to deliver a concise and accurate response to a user's query."
        "Your answer must be precise, of high-quality, and written by an expert."
        "It is EXTREMELY IMPORTANT to directly answer the query."
        'NEVER say "based on the search results" or start your answer with a heading or title.'
        "Get straight to the point."
        "If you don't know the answer or the premise is incorrect, explain why. If the results are empty or unhelpful, answer the query as well as you can with existing knowledge."
        f"Your answer MUST be written in {context.locale}."
        "## Guidelines\n"
        "If there is insufficient information to answer the question, consider attempting the following options.\n"
        "1. Retry a function with different arguments.\n"
        "2. Try another function.\n"
        "3. Ask a question to the user.\n"
        "4. Do nothing.\n"
        "Priority from top to bottom.\n"
    )

    if avatar is not None:
        instructions.append(f"## Instruction\n{avatar.instruction}\n")

    messages = []
    if len(instructions) > 0:
        system_message = {"role": "system", "content": "\n".join(instructions)}
        messages.append(system_message)

    for i, message in enumerate(replies["messages"]):
        role = (
            "assistant"
            if message.get("user", event.get("username")) == s.slack_bot_member_id
            else "user"
        )
        if role == "assistant" and i == 0:
            role = "system"  # bot's first message is a system message

        contents = []

        content = message["text"].replace(f"<@{s.slack_bot_member_id}>", "").strip()

        for attachment in message.get("attachments", []):
            if quote_url := attachment.get("from_url"):
                content += "\n"
                content += f"> {quote_url}"

        files = message.get("files", [])
        for file in files:
            url = file.get("url_private_download")
            if url is None:
                continue

            mimetype = file.get("mimetype", "")
            if mimetype.startswith("image/") and model.image_url_supported:
                headers = {
                    "Authorization": f"Bearer {s.slack_bot_token}",
                }
                try:
                    async with aiohttp.ClientSession(
                        headers=headers, trust_env=True
                    ) as session:
                        async with session.get(url, allow_redirects=True) as response:
                            if response.status == 200:
                                b = await response.read()
                                if len(b) < 20 * 1024 * 1024:
                                    # Image URLs are only allowed for messages with role 'user'
                                    role = "user"
                                    contents.append(
                                        {
                                            "type": "image_url",
                                            "image_url": {
                                                "url": f"data:{mimetype};base64,{base64.b64encode(b).decode()}",
                                            },
                                        }
                                    )
                                    continue
                except (
                        aiohttp.ClientConnectionError,  # ECONNREFUSED, EPIPE, ECONNRESET
                        aiohttp.client_exceptions.ServerDisconnectedError,
                        asyncio.TimeoutError,
                ) as e:
                    raise cortex.exceptions.RetryableError(e) from e

            content += "\n"
            content += url

        if content != "":
            contents.append(
                {
                    "type": "text",
                    "text": content,
                }
            )

        if len(contents) == 0:
            continue

        messages.append(
            {
                "role": role,
                "content": contents,
            },
        )

    if len(messages) == 0:
        return

    loaded_messages = await context.load_messages()
    metadata_message = {
        "role": "system",
        "content": (
            "<metadata>\n"
            f"Slack channel: {event['channel']}\n"
            f"Now: {datetime.datetime.now().astimezone().isoformat()}\n"
            "</metadata>\n"
        )
    }
    messages = [*messages[:-1], *loaded_messages, metadata_message, *messages[-1:]]
    context.current_messages = messages

    progress = await retryable_client.chat_postMessage(  # Special Tier (1 per second per channel)
        channel=event["channel"],
        thread_ts=event.get("thread_ts") or event["ts"],
        text=":hourglass:",
        username=avatar.name if avatar is not None else None,
        icon_url=avatar.icon_url if avatar is not None else None,
    )

    try:
        context.progress = progress
        await context.acquire_budget(s.loop_budget)

        stream = not s.disable_streaming and model.stream_supported
        try:
            async with playwright.async_api.async_playwright() as pw:
                if s.chrome_devtools_protocol_url is not None:
                    browser = await pw.chromium.connect_over_cdp(
                        s.chrome_devtools_protocol_url
                    )
                else:
                    browser = await pw.chromium.launch(
                        proxy={"server": os.getenv("HTTPS_PROXY"), "bypass": "*"}
                        if os.getenv("HTTPS_PROXY")
                        else None,
                    )
                response = await bot.agent.root_agent.RootAgent(
                    browser=browser,
                    embedding_retrieval_url=s.embedding_retrieval_url,
                    grafana_mcp_url=s.grafana_mcp_url,
                    playwright_mcp_url=s.playwright_mcp_url,
                    github_token=s.github_token,
                    slack_token=s.slack_bot_token,
                    google_credentials=google.oauth2.credentials.Credentials.from_authorized_user_info(
                        {
                            "client_id": s.google_client_id,
                            "client_secret": s.google_client_secret,
                            "refresh_token": s.google_pre_issued_refresh_token,
                            "token_uri": "https://accounts.google.com/o/oauth2/token",
                            "scopes": cortex.GOOGLE_OAUTH_SCOPES,
                        }
                    ),
                    bing_subscription_key=s.bing_subscription_key,
                    google_custom_search_api_key=s.google_custom_search_api_key,
                    google_custom_search_engine_id=s.google_custom_search_engine_id,
                    model=model,
                    reasoning_effort=reasoning_effort,
                    verbosity=verbosity,
                    encoder=encoder,
                    image_model=s.image_model,
                    audio_model=s.audio_model,
                ).chat_completion_loop(
                    messages,
                    context,
                    stream=stream,
                )
        except openai.RateLimitError:
            await retryable_client.chat_update(  # Tier 3 (50+ per minute)
                channel=progress["channel"],
                ts=progress["ts"],
                text=bot.slack.i18n.translate(
                    "You have reached your OpenAI API billing limit.\nPlease wait a moment and try again.",
                    locale=context.locale,
                ),
            )
            return
        except openai.BadRequestError as e:
            match e.code:
                case "billing_hard_limit_reached":
                    await retryable_client.chat_update(  # Tier 3 (50+ per minute)
                        channel=progress["channel"],
                        ts=progress["ts"],
                        text=bot.slack.i18n.translate(
                            "You have reached your OpenAI API billing limit.\nPlease wait a moment and try again.",
                            locale=context.locale,
                        ),
                    )
                    return
                case "context_length_exceeded":
                    await retryable_client.chat_update(  # Tier 3 (50+ per minute)
                        channel=progress["channel"],
                        ts=progress["ts"],
                        text=bot.slack.i18n.translate(
                            "The maximum number of conversations has been exceeded.\nPlease create a new thread.",
                            locale=context.locale,
                        ),
                    )
                    return
                case "content_policy_violation":
                    await retryable_client.chat_update(  # Tier 3 (50+ per minute)
                        channel=progress["channel"],
                        ts=progress["ts"],
                        text=bot.slack.i18n.translate(
                            "Your message contains violent or explicit.",
                            locale=context.locale,
                        ),
                    )
                    return
            raise e
        except cortex.exceptions.InsufficientBudgetError:
            await retryable_client.chat_update(  # Tier 3 (50+ per minute)
                channel=progress["channel"],
                ts=progress["ts"],
                text=bot.slack.i18n.translate(
                    "Could not resolve complex task.",
                    locale=context.locale,
                ),
            )
            return

        tasks = []

        if stream:
            queue: collections.abc.MutableSequence[str] = []

            async def chat_update():
                if len(queue) > 0:
                    fragment_text = bot.slack.transform_message(
                        "".join(queue),
                        bot.slack.SlackMarkdownRenderer(
                            cortex.URLShortener(s.url_shortener_url)
                        ),
                    )
                    try:
                        if (
                            len(fragment_text.encode("utf-8"))
                            > bot.slack.CHAT_UPDATE_MAX_BYTES
                        ):
                            return

                        await client.chat_update(  # Tier 3 (50+ per minute)
                            channel=progress["channel"],
                            ts=progress["ts"],
                            text=fragment_text,
                        )
                    except slack_sdk.errors.SlackApiError as ie:
                        if ie.response.status_code == 429 or "error" not in ie.response:
                            raise cortex.exceptions.RetryableError(ie) from ie
                        else:
                            if ie.response["error"] in ["no_text", "msg_too_long"]:
                                return
                            raise ie

            throttled_chat_update = cortex.throttled_function.ThrottledFunction(
                chat_update,
                s.streaming_throttled_interval,
            )
            try:
                async for r in response:
                    if len(r.choices) == 0:
                        continue

                    choice = r.choices[0]
                    match choice.finish_reason:
                        case "length":
                            await retryable_client.chat_update(  # Tier 3 (50+ per minute)
                                channel=progress["channel"],
                                ts=progress["ts"],
                                text=bot.slack.i18n.translate(
                                    "The maximum number of conversations has been exceeded.\nPlease create a new thread.",
                                    locale=context.locale,
                                ),
                            )
                            return
                        case "content_filter":
                            await (
                                retryable_client.chat_update(  # Tier 3 (50+ per minute)
                                    channel=progress["channel"],
                                    ts=progress["ts"],
                                    text=bot.slack.i18n.translate(
                                        "Your message contains violent or explicit.",
                                        locale=context.locale,
                                    ),
                                )
                            )
                            return
                        case "tool_calls" | "stop" | None:
                            pass
                        case _ as finish_reason:
                            raise ValueError(f"Unknown finish_reason: {finish_reason}")

                    if choice.delta.content is None:
                        continue
                    queue.append(choice.delta.content)
                    await throttled_chat_update.execute()
            except (
                    httpx.RemoteProtocolError,
                    httpx.ReadTimeout,
                    openai.APIConnectionError,
            ) as e:
                raise cortex.exceptions.RetryableError(e) from e
            except openai.APIError as e:
                match e.code:
                    case "server_error" | "rate_limit_exceeded":
                        raise cortex.exceptions.RetryableError(e) from e
                raise e

            context.increment_completion_tokens(
                s.model, len(encoder.encode("".join(queue)))
            )

            bot.telemetry.logger.info(
                json.dumps(
                    {
                        "user": event.get("user", event.get("username", "")),
                        "team": event.get("team", event.get("user_team", "")),
                        "channel": event["channel"],
                        "thread": event.get("thread_ts") or event["ts"],
                        "prompt": messages,
                        "completion": "".join(queue),
                    },
                    ensure_ascii=False,
                ),
            )

            async def immediately_execute_with_backoff_loop(
                next_delay: int, max_delay: int | None = None
            ):
                if max_delay is None:
                    delay = next_delay
                else:
                    delay = min(next_delay, max_delay)

                try:
                    await throttled_chat_update.immediately_execute()
                except cortex.exceptions.RetryableError as ie:
                    cause = ie.__cause__
                    match cause:
                        case slack_sdk.errors.SlackApiError as cause:
                            if (
                                cause.response.status_code == 429
                                and "Retry-After" in cause.response.headers
                            ):
                                delay = int(cause.response.headers["Retry-After"])

                    await asyncio.sleep(delay * random.random())

                    if next_delay <= (2 ** 63 - 1) / 2:
                        next_delay *= 2
                    else:
                        next_delay = 2 ** 63 - 1
                    return await immediately_execute_with_backoff_loop(
                        next_delay=next_delay, max_delay=max_delay
                    )

            text = bot.slack.transform_message(
                "".join(queue),
                bot.slack.SlackMarkdownRenderer(
                    cortex.URLShortener(s.url_shortener_url)
                ),
            )
            if len(text.encode("utf-8")) > bot.slack.CHAT_UPDATE_MAX_BYTES:

                async def _task():
                    await retryable_client.chat_delete(  # Tier 3 (50+ per minute)
                        channel=progress["channel"],
                        ts=progress["ts"],
                    )
                    new = await retryable_client.chat_postMessage(  # Special Tier (1 per second per channel)
                        channel=event["channel"],
                        thread_ts=event.get("thread_ts") or event["ts"],
                        text=text,
                        username=avatar.name if avatar is not None else None,
                        icon_url=avatar.icon_url if avatar is not None else None,
                    )
                    if s.callback_reaction is not None:
                        await retryable_client.reactions_add(  # Tier 3 (50+ per minute)
                            channel=event["channel"],
                            timestamp=new["ts"],
                            name=s.callback_reaction,
                        )

                tasks.append(_task())
            else:

                async def _task():
                    if throttled_chat_update.is_throttled:
                        tasks.append(
                            immediately_execute_with_backoff_loop(
                                next_delay=1, max_delay=10
                            )
                        )
                    if s.callback_reaction is not None:
                        await retryable_client.reactions_add(  # Tier 3 (50+ per minute)
                            channel=event["channel"],
                            timestamp=progress["ts"],
                            name=s.callback_reaction,
                        )

                tasks.append(_task())
        else:
            choice = response.choices[0]
            match choice.finish_reason:
                case "length":
                    await retryable_client.chat_update(  # Tier 3 (50+ per minute)
                        channel=progress["channel"],
                        ts=progress["ts"],
                        text=bot.slack.i18n.translate(
                            "The maximum number of conversations has been exceeded.\nPlease create a new thread.",
                            locale=context.locale,
                        ),
                    )
                    return
                case "content_filter":
                    await retryable_client.chat_update(  # Tier 3 (50+ per minute)
                        channel=progress["channel"],
                        ts=progress["ts"],
                        text=bot.slack.i18n.translate(
                            "Your message contains violent or explicit.",
                            locale=context.locale,
                        ),
                    )
                    return
                case "tool_calls" | "stop" | None:
                    pass
                case _ as finish_reason:
                    raise ValueError(f"Unknown finish_reason: {finish_reason}")

            context.increment_completion_tokens(
                s.model, response.usage.completion_tokens
            )

            text = bot.slack.transform_message(
                choice.message.content,
                bot.slack.SlackMarkdownRenderer(
                    cortex.URLShortener(s.url_shortener_url)
                ),
            )
            if len(text.encode("utf-8")) > bot.slack.CHAT_UPDATE_MAX_BYTES:

                async def _task():
                    await retryable_client.chat_delete(  # Tier 3 (50+ per minute)
                        channel=progress["channel"],
                        ts=progress["ts"],
                    )
                    new = await retryable_client.chat_postMessage(  # Special Tier (1 per second per channel)
                        channel=event["channel"],
                        thread_ts=event.get("thread_ts") or event["ts"],
                        text=text,
                        username=avatar.name if avatar is not None else None,
                        icon_url=avatar.icon_url if avatar is not None else None,
                    )
                    if s.callback_reaction is not None:
                        await retryable_client.reactions_add(  # Tier 3 (50+ per minute)
                            channel=event["channel"],
                            timestamp=new["ts"],
                            name=s.callback_reaction,
                        )

                tasks.append(_task())
            else:

                async def _task():
                    await retryable_client.chat_update(  # Tier 3 (50+ per minute)
                        channel=progress["channel"],
                        ts=progress["ts"],
                        text=text,
                    )
                    if s.callback_reaction is not None:
                        await retryable_client.reactions_add(  # Tier 3 (50+ per minute)
                            channel=event["channel"],
                            timestamp=progress["ts"],
                            name=s.callback_reaction,
                        )

                tasks.append(_task())

        prompt_tokens = context.prompt_tokens
        completion_tokens = context.completion_tokens
        embedding_tokens = context.embedding_tokens
        generated_images = context.generated_images
        processed_audio_seconds = context.processed_audio_seconds
        converted_text_characters = context.converted_text_characters

        price = calculate_model_price(
            prompt_tokens,
            completion_tokens,
            embedding_tokens,
            generated_images,
            processed_audio_seconds,
            converted_text_characters,
        )
        # Slack API does not allow USLACKBOT to post ephemeral messages
        if (
            event.get("user", event.get("username", "")) != "USLACKBOT"
            and "user" in event
        ):
            retryable_original_client = bot.slack.RetryableAsyncWebClient(
                context.original_client
            )
            text = bot.slack.i18n.translate(
                "The price for this response was {price}USD.",
                locale=context.locale,
            ).format(
                price=price,
            )

            for function in context.call_stack:
                if function.escalation is None:
                    continue

                escalation_message = function.escalation(context)
                if escalation_message is None:
                    continue

                text += "\n\n"
                text += escalation_message

            tasks.append(
                retryable_original_client.chat_postEphemeral(  # Tier 4 (100+ per minute)
                    channel=event["channel"],
                    thread_ts=event.get("thread_ts") or event["ts"],
                    user=event.get("user", event.get("username", "")),
                    text=text,
                    blocks=[
                        {
                            "type": "section",
                            "text": {
                                "type": "mrkdwn",
                                "text": text,
                            },
                        },
                        bot.slack.reporter.build_reporting_block(context),
                    ],
                ),
            )

        consumed_messages = copy.deepcopy(messages)
        for message in consumed_messages:
            content = message.get("content")
            if not isinstance(content, collections.abc.MutableSequence):
                continue
            message["content"] = [
                c
                for c in content
                if not (
                    isinstance(c, collections.abc.Mapping)
                    and c.get("type") == "image_url"
                )
            ]

        messages_token = len(
            encoder.encode(
                "".join(
                    [
                        json.dumps(message, ensure_ascii=False, separators=(",", ":"))
                        for message in consumed_messages
                    ]
                )
            )
        )
        if messages_token > s.model.max_tokens:
            # Slack API does not allow USLACKBOT to post ephemeral messages
            if (
                event.get("user", event.get("username")) != "USLACKBOT"
                and "user" in event
            ):
                tasks.append(
                    retryable_client.chat_postEphemeral(  # Tier 4 (100+ per minute)
                        channel=event["channel"],
                        thread_ts=event.get("thread_ts") or event["ts"],
                        user=event.get("user", event.get("username", "")),
                        text=bot.slack.i18n.translate(
                            ":warning: The conversation are very long.\nIt is recommend to create a new thread due to the lower accuracy.",
                            locale=context.locale,
                        ),
                    ),
                )

        sum_prompt_tokens = sum(prompt_tokens.values())
        sum_completion_tokens = sum(completion_tokens.values())
        sum_embedding_tokens = sum(embedding_tokens.values())
        sum_generated_images = sum(generated_images.values())

        tasks.append(
            rate_limiter.take(
                rate_limiter_key, sum_prompt_tokens + sum_completion_tokens
            )
        )
        image_generation_cost = 0
        for image_model, n in generated_images:
            image_generation_cost += int(
                (image_model.price / s.model.prices["price_per_prompt"]) * n
            )
        tasks.append(rate_limiter.take(rate_limiter_key, image_generation_cost))

        await asyncio.gather(*tasks)
    except cortex.exceptions.RetryableError as e:
        await retryable_client.chat_update(  # Tier 3 (50+ per minute)
            channel=progress["channel"],
            ts=progress["ts"],
            text=bot.slack.i18n.translate(
                "A temporary error has occurred.\nPlease try again.",
                locale=context.locale,
            ),
        )
        raise e
    except Exception as e:
        await retryable_client.chat_update(  # Tier 3 (50+ per minute)
            channel=progress["channel"],
            ts=progress["ts"],
            text=bot.slack.i18n.translate(
                "An unknown error has occurred",
                locale=context.locale,
            ),
        )
        raise e

    bot.telemetry.logger.info(
        json.dumps(
            {
                "team": event.get("team", event.get("user_team", "")),
                "user": event.get("user", event.get("username", "")),
                "channel": event["channel"],
                "thread": event.get("thread_ts") or event["ts"],
                "prompt_tokens": sum_prompt_tokens,
                "completion_tokens": sum_completion_tokens,
                "embedding_tokens": sum_embedding_tokens,
                "generated_images": sum_generated_images,
            }
        ),
    )

    for model, tokens in prompt_tokens.items():
        get_processed_tokens_total().add(
            tokens,
            attributes={
                "usage": "prompt",
                "model": model,
                "slack.team": event.get("team", event.get("user_team", "")),
                "slack.user": event.get("user", event.get("username", "")),
                "slack.channel": event["channel"],
            },
        )
    for model, tokens in completion_tokens.items():
        get_processed_tokens_total().add(
            tokens,
            attributes={
                "usage": "completion",
                "model": model,
                "slack.team": event.get("team", event.get("user_team", "")),
                "slack.user": event.get("user", event.get("username", "")),
                "slack.channel": event["channel"],
            },
        )
    for model, tokens in embedding_tokens.items():
        get_processed_tokens_total().add(
            tokens,
            attributes={
                "usage": "embedding",
                "model": model,
                "slack.team": event.get("team", event.get("user_team", "")),
                "slack.user": event.get("user", event.get("username", "")),
                "slack.channel": event["channel"],
            },
        )
    for model, images in generated_images.items():
        get_generated_images_total().add(
            images,
            attributes={
                "model": model,
                "slack.team": event.get("team", event.get("user_team", "")),
                "slack.user": event.get("user", event.get("username", "")),
                "slack.channel": event["channel"],
            },
        )


@bolt.event("message")
async def handle_message_im(
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

    if event["channel_type"] != "im":
        return

    with bot.telemetry.tracer.start_as_current_span(
        "handle_message",
        attributes={
            "model": s.model,
            "slack.team": event.get("team", event.get("user_team", "")),
            "slack.user": event.get("user", event.get("username", "")),
            "slack.channel": event["channel"],
            "slack.thread": event.get("thread_ts") or event["ts"],
        },
    ):
        retryable_client = bot.slack.RetryableAsyncWebClient(client)

        if "subtype" in event and event["subtype"] not in [
            "file_share",
            "bot_message",
            "thread_broadcast",
        ]:
            return

        context = bot.slack.SlackContext(
            context_id=event.get("thread_ts") or event["ts"],
            memory_type=s.memory_type,
            embedding_model=s.embedding_model,
            encoder=encoder,
            original_client=client,
            client=retryable_client,
            event=event,
            channel={},
            user={},
            progress=None,
        )

        if channel_cache.get(event["channel"]) is None:
            channel_info = (
                await retryable_client.conversations_info(  # Tier 3 (50+ per minute)
                    channel=event["channel"],
                )
            )
            channel_cache[event["channel"]] = channel_info["channel"]

        context.channel = channel_cache[event["channel"]]

        # Bot inherits the owner of the channel in the direct message
        if (
            user := context.channel.get("user", context.channel.get("username"))
        ) is not None:
            if user_cache.get(user) is None:
                user_info = (
                    await retryable_client.users_info(  # Tier 4 (100+ per minute)
                        user=user,
                    )
                )
                user_cache[user] = user_info["user"]

            context.user = user_cache[user]

            if not s.allow_restricted_user or s.allow_teams or s.allow_email_domains:
                if not validate_user(context.user):
                    await say(
                        bot.slack.i18n.translate(
                            "You are not available.",
                            locale=context.locale,
                        ),
                        thread_ts=event.get("thread_ts") or event["ts"],
                    )
                    return

        await handle_event_with_retry(client, retryable_client, event, context)


@bolt.event("app_mention")
async def handle_app_mention(
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

    with bot.telemetry.tracer.start_as_current_span(
        "handle_app_mention",
        attributes={
            "model": s.model,
            "slack.team": event.get("team", event.get("user_team", "")),
            "slack.user": event.get("user", event.get("username", "")),
            "slack.channel": event["channel"],
            "slack.thread": event.get("thread_ts") or event["ts"],
        },
    ):
        original_client = client

        api_client = await api_client_factory.construct(event["channel"])
        if api_client is not None:
            client = api_client

        retryable_client = bot.slack.RetryableAsyncWebClient(client)

        context = bot.slack.SlackContext(
            context_id=event.get("thread_ts") or event["ts"],
            memory_type=s.memory_type,
            embedding_model=s.embedding_model,
            encoder=encoder,
            original_client=original_client,
            client=retryable_client,
            event=event,
            channel={},
            user={},
            progress=None,
        )

        if channel_cache.get(event["channel"]) is None:
            channel_info = (
                await retryable_client.conversations_info(  # Tier 3 (50+ per minute)
                    channel=event["channel"],
                )
            )
            channel_cache[event["channel"]] = channel_info["channel"]

        context.channel = channel_cache[event["channel"]]

        # Bot inherits the creator of the channel
        if (user := event.get("user", event.get("username"))) is not None or (
            user := context.channel.get("creator")
        ) is not None:
            if user_cache.get(user) is None:
                user_info = (
                    await retryable_client.users_info(  # Tier 4 (100+ per minute)
                        user=user,
                    )
                )
                user_cache[user] = user_info["user"]

            context.user = user_cache[user]

            if not s.allow_restricted_user or s.allow_teams or s.allow_email_domains:
                if not validate_user(context.user):
                    await say(
                        bot.slack.i18n.translate(
                            "You are not available.",
                            locale=context.locale,
                        ),
                        thread_ts=event.get("thread_ts") or event["ts"],
                    )
                    return

        if not s.allow_ext_shared_channel:
            if body["is_ext_shared_channel"]:
                await say(
                    bot.slack.i18n.translate(
                        "Not available on this channel.",
                        locale=context.locale,
                    ),
                    thread_ts=event.get("thread_ts") or event["ts"],
                )
                return

        if s.allow_channels:
            if event["channel"] not in s.allow_channels:
                await say(
                    bot.slack.i18n.translate(
                        "Not available on this channel.",
                        locale=context.locale,
                    ),
                    thread_ts=event.get("thread_ts") or event["ts"],
                )
                return

        await handle_event_with_retry(client, retryable_client, event, context)


@bolt.error
async def custom_error_handler(error: Exception):
    if isinstance(error, cortex.exceptions.RetryableError):
        get_internal_errors_total().add(1, attributes={"retryable": "true"})
        bot.telemetry.logger.error(error)
    elif isinstance(error, Exception):
        get_internal_errors_total().add(1, attributes={"retryable": "false"})
        bot.telemetry.logger.error(error, exc_info=error)


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
            class AsyncSocketModeHandlerWrapper(
                slack_bolt.adapter.socket_mode.async_handler.AsyncSocketModeHandler
            ):
                async def handle(
                    self,
                    client: slack_sdk.socket_mode.aiohttp.SocketModeClient,
                    req: slack_sdk.socket_mode.request.SocketModeRequest,
                ) -> None:
                    event = req.payload.get("event")

                    if event is None:
                        return await super().handle(client, req)

                    with bot.telemetry.tracer.start_as_current_span(
                        "handle",
                        attributes={
                            "model": s.model,
                            "slack.team": event["team"],
                            "slack.user": event.get("user", event.get("username", "")),
                            "slack.channel": event["channel"],
                        }
                        if "team" in event
                        else {
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
                    t
                    for t in asyncio.all_tasks()
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
