import asyncio
import collections.abc
import contextlib
import datetime
import json
import logging
import os
import time
import typing
import uuid

import aredis_om
import fastapi
import google.oauth2.credentials
import httpx
import openai
import openai.types.chat
import openai.types.shared_params
import opentelemetry.context
import opentelemetry.exporter.otlp.proto.grpc.trace_exporter
import opentelemetry.exporter.prometheus
import opentelemetry.instrumentation.aiohttp_client
import opentelemetry.instrumentation.fastapi
import opentelemetry.instrumentation.httpx
import opentelemetry.instrumentation.redis
import opentelemetry.instrumentation.requests
import opentelemetry.metrics
import opentelemetry.sdk.metrics
import opentelemetry.sdk.resources
import opentelemetry.sdk.trace.export
import opentelemetry.trace
import playwright.async_api
import prometheus_client
import pydantic
import pythonjsonlogger.jsonlogger
import redis
import redis.asyncio
import redis.asyncio.retry
import redis.backoff
import redis.exceptions
import sse_starlette.sse
import tiktoken
import websockets.asyncio.client

import api.agent.root_agent
import api.settings
import api.telemetry
import cortex.exceptions
import cortex.llm.openai.agent.memory
import cortex.llm.openai.model
import cortex.rate_limit

s = api.settings.Settings()

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

    with api.telemetry.tracer.start_as_current_span("startup"):
        match s.memory_type:
            case cortex.llm.openai.agent.MemoryType.Redis:
                cortex.llm.openai.agent.memory.RedisMemory.Meta.database = (
                    redis.asyncio.Redis(
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
                )
                await aredis_om.Migrator().run()


@contextlib.asynccontextmanager
async def lifespan(_app: fastapi.FastAPI):
    await startup()
    yield


app = fastapi.FastAPI(lifespan=lifespan)
opentelemetry.instrumentation.fastapi.FastAPIInstrumentor.instrument_app(app)


def format_reset_time(reset_timestamp: int) -> str:
    seconds_until_reset = max(0, reset_timestamp - int(time.time()))

    if seconds_until_reset < 60:
        return f"{seconds_until_reset}s"
    elif seconds_until_reset < 3600:
        minutes = seconds_until_reset // 60
        seconds = seconds_until_reset % 60
        return f"{minutes}m{seconds}s"
    else:
        hours = seconds_until_reset // 3600
        remaining = seconds_until_reset % 3600
        minutes = remaining // 60
        seconds = remaining % 60
        if minutes > 0:
            return f"{hours}h{minutes}m{seconds}s"
        else:
            return f"{hours}h{seconds}s"


def rate_limit_headers(
    rate_limit_info: cortex.rate_limit.RateLimitInfo,
) -> collections.abc.Mapping[str, str]:
    headers = {
        "X-RateLimit-Limit-Tokens": str(rate_limit_info.limit),
        "X-RateLimit-Remaining-Tokens": str(rate_limit_info.remaining),
        "X-RateLimit-Reset": format_reset_time(rate_limit_info.reset_timestamp),
    }

    if rate_limit_info.retry_after is not None:
        headers["Retry-After"] = str(rate_limit_info.retry_after)

    return headers


@app.middleware("http")
async def override_server_error_middleware(request: fastapi.Request, call_next):
    try:
        return await call_next(request)
    except cortex.exceptions.RetryableError as e:
        api.telemetry.logger.error(e)
        return fastapi.responses.JSONResponse(
            {"error": {"message": "Service Unavailable", "type": "server_error"}},
            status_code=503,
        )
    except Exception as e:
        api.telemetry.logger.error(e, exc_info=True)
        return fastapi.responses.JSONResponse(
            {"error": {"message": "Internal Server Error", "type": "server_error"}},
            status_code=500,
        )


global_processed_tokens_total: opentelemetry.metrics.Counter | None = None


def get_processed_tokens_total() -> opentelemetry.metrics.Counter:
    global global_processed_tokens_total
    if global_processed_tokens_total is None:
        global_processed_tokens_total = api.telemetry.meter.create_counter(
            "processed_tokens_total",
            description="Total number of tokens processed",
        )
    return global_processed_tokens_total


global_generated_images_total: opentelemetry.metrics.Counter | None = None


def get_generated_images_total() -> opentelemetry.metrics.Counter:
    global global_generated_images_total
    if global_generated_images_total is None:
        global_generated_images_total = api.telemetry.meter.create_counter(
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
                global_rate_limiter = cortex.rate_limit.RedisSlidingRateLimiter(
                    redis.asyncio.Redis(
                        host=s.redis_host,
                        port=s.redis_port,
                        retry=redis.asyncio.retry.Retry(
                            redis.backoff.ExponentialBackoff(), 3
                        ),
                        retry_on_error=[
                            redis.exceptions.ConnectionError,
                            redis.exceptions.ReadOnlyError,
                        ],
                    ),
                    interval_seconds=s.rate_limit_interval_seconds,
                )
            case _:
                raise NotImplementedError
    return global_rate_limiter


class TextTypedContent(pydantic.BaseModel):
    type: typing.Literal["text"]
    text: str


class MessageContentTypedImageUrl(pydantic.BaseModel):
    url: str


class ImageUrlTypedContent(pydantic.BaseModel):
    type: typing.Literal["image_url"]
    image_url: MessageContentTypedImageUrl


class AudioTypedContent(pydantic.BaseModel):
    type: typing.Literal["audio"]
    id: str


TypedContent = typing.Union[TextTypedContent, ImageUrlTypedContent, AudioTypedContent]


class Message(pydantic.BaseModel):
    role: str
    content: typing.Union[str, collections.abc.Sequence[TypedContent]]


class APIContext(cortex.llm.openai.agent.Context):
    user: str

    def __init__(
        self,
        context_id: str,
        memory_type: cortex.llm.openai.agent.MemoryType,
        embedding_model: cortex.llm.openai.model.EmbeddingModel,
        encoder: tiktoken.Encoding,
        user: str,
    ):
        super().__init__(context_id, memory_type, embedding_model, encoder)
        self.user = user

    @property
    def capability(self) -> cortex.llm.openai.agent.Capability:
        return cortex.llm.openai.agent.Capability.ALL

    @property
    def limit(self) -> int | None:
        return None

    async def report_progress(
        self, message: str, stage: cortex.llm.openai.agent.ProgressStage
    ):
        pass


async def count_token(
    context: APIContext,
    rate_limiter: cortex.rate_limit.RateLimiter,
    rate_limiter_key: str,
):
    prompt_tokens = context.prompt_tokens
    completion_tokens = context.completion_tokens
    embedding_tokens = context.embedding_tokens

    sum_prompt_tokens = sum(prompt_tokens.values())
    sum_completion_tokens = sum(completion_tokens.values())
    sum_embedding_tokens = sum(embedding_tokens.values())
    await rate_limiter.take(rate_limiter_key, sum_prompt_tokens + sum_completion_tokens)

    api.telemetry.logger.info(
        json.dumps(
            {
                "prompt_tokens": sum_prompt_tokens,
                "completion_tokens": sum_completion_tokens,
                "embedding_tokens": sum_embedding_tokens,
                "user": context.user,
            }
        ),
    )

    for model, tokens in prompt_tokens.items():
        get_processed_tokens_total().add(
            tokens,
            attributes={
                "usage": "prompt",
                "model": model,
                "user": context.user,
            },
        )
    for model, tokens in completion_tokens.items():
        get_processed_tokens_total().add(
            tokens,
            attributes={
                "usage": "completion",
                "model": model,
                "user": context.user,
            },
        )
    for model, tokens in embedding_tokens.items():
        get_processed_tokens_total().add(
            tokens,
            attributes={
                "usage": "embedding",
                "model": model,
                "user": context.user,
            },
        )


# https://platform.openai.com/docs/api-reference/chat/create
class V1ChatCompletions(pydantic.BaseModel):
    messages: collections.abc.Sequence[Message] = pydantic.Field(
        ...,
        description="A list of messages comprising the conversation so far. [Example Python code](https://cookbook.openai.com/examples/how_to_format_inputs_to_chatgpt_models).",
    )
    model: cortex.llm.openai.model.CompletionModel | None = pydantic.Field(
        None,
        description="ID of the model to use. See the [model endpoint compatibility](https://platform.openai.com/docs/models/model-endpoint-compatibility) table for details on which models work with the Chat API. We **strongly recommend using the latest model ID** such as `gpt-3.5-turbo` for backwards compatibility with future model releases.",
    )
    reasoning_effort: openai.types.chat.ChatCompletionReasoningEffort | None = (
        pydantic.Field(
            None,
            description="Constrains effort on reasoning for reasoning models. Currently supported values are low, medium, and high. Reducing reasoning effort can result in faster responses and fewer tokens used on reasoning in a response.",
        )
    )
    stream: bool = pydantic.Field(
        False,
        description="If set, partial message deltas will be sent, like in ChatGPT. Tokens will be sent as data-only [server-sent events](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events/Using_server-sent_events#Event_stream_format) as they become available, with the stream terminated by a data: [DONE] message. [Example Python code](https://cookbook.openai.com/examples/how_to_stream_completions).",
    )
    n: int = pydantic.Field(
        1,
        description="How many chat completion choices to generate for each input message. Note that you will be charged based on the number of generated tokens across all of the choices. Keep n as 1 to minimize costs.",
    )
    response_format: (
        openai.types.shared_params.ResponseFormatText
        | openai.types.shared_params.ResponseFormatJSONObject
        | openai.types.shared_params.ResponseFormatJSONSchema
        | None
    ) = pydantic.Field(
        None,
        description=(
            "An object specifying the format that the model must output. Compatible with GPT-4o, GPT-4o mini, GPT-4 Turbo and all GPT-3.5 Turbo models newer than gpt-3.5-turbo-1106.\n"
            "\n"
            """Setting to `{ "type": "json_schema", "json_schema": {...} }` enables Structured Outputs which guarantees the model will match your supplied JSON schema. Learn more in the Structured Outputs guide.\n"""
            "\n"
            """Setting to `{ "type": "json_object" }` enables JSON mode, which guarantees the message the model generates is valid JSON.\n"""
            "\n"
            """Important: when using JSON mode, you must also instruct the model to produce JSON yourself via a system or user message. Without this, the model may generate an unending stream of whitespace until the generation reaches the token limit, resulting in a long-running and seemingly "stuck" request. Also note that the message content may be partially cut off if finish_reason="length", which indicates the generation exceeded max_tokens or the conversation exceeded the max context length.\n"""
        ),
    )


@app.post("/v1/chat/completions")
async def completions(
    body: V1ChatCompletions,
    cortex_mode: bool = fastapi.Header(default=False),
    x_auth_request_user: str = fastapi.Header(default="anonymous"),
):
    context = APIContext(
        context_id=uuid.uuid4().hex,
        memory_type=s.memory_type,
        embedding_model=s.embedding_model,
        encoder=encoder,
        user=x_auth_request_user,
    )

    rate_limiter = await get_rate_limiter()
    rate_limiter_key = x_auth_request_user
    limit = context.limit or s.rate_limit_per_interval

    rate_limit_info = await rate_limiter.remaining(rate_limiter_key, limit)

    if rate_limit_info.is_exceeded:
        retry_after = max(1, rate_limit_info.reset_timestamp - int(time.time()))
        rate_limit_info.retry_after = retry_after

        return fastapi.responses.JSONResponse(
            {"error": {"message": "Too Many Requests", "type": "rate_limit_error"}},
            status_code=429,
            headers=rate_limit_headers(rate_limit_info),
        )

    try:
        if cortex_mode:
            instructions = [
                "Your task is to deliver a concise and accurate response to a user's query."
                "Your answer must be precise, of high-quality, and written by an expert."
                "It is EXTREMELY IMPORTANT to directly answer the query."
                'NEVER say "based on the search results" or start your answer with a heading or title.'
                "Get straight to the point."
                "Your answer must be written in the same language as the query, even if language preference is different."
                "If you don't know the answer or the premise is incorrect, explain why. If the results are empty or unhelpful, answer the query as well as you can with existing knowledge."
                "## Guidelines\n"
                "If there is insufficient information to answer the question, consider attempting the following options.\n"
                "1. Retry a function with different arguments.\n"
                "2. Try another function.\n"
                "3. Ask a question to the user.\n"
                "4. Do nothing.\n"
                "Priority from top to bottom.\n"
                "## Metadata\n"
                f"Now: {datetime.datetime.now().astimezone().isoformat()}\n"
            ]
            if s.system_prompt is not None:
                instructions.append(s.system_prompt)

            messages = []
            if len(instructions) > 0:
                system_message = {"role": "system", "content": "\n".join(instructions)}
                messages.append(system_message)

            for message in body.messages:
                messages.append(message.model_dump())

            if len(messages) == 0:
                return

            await context.acquire_budget(s.loop_budget)

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
                responses = await asyncio.gather(
                    *[
                        api.agent.root_agent.RootAgent(
                            browser=browser,
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
                            model=body.model or s.model,
                            reasoning_effort=body.reasoning_effort or "medium",
                            encoder=context.encoder,
                        ).chat_completion_loop(
                            messages,
                            context,
                            response_format=body.response_format,
                            stream=body.stream,
                        )
                        for _ in range(body.n)
                    ]
                )

            if body.stream:

                async def sse(responses) -> collections.abc.AsyncIterable[typing.Any]:
                    async def wrapper(index, response):
                        try:
                            async for r in response:
                                for choice in r.choices:
                                    choice.index = index
                                    if choice.delta.content is not None:
                                        context.increment_completion_tokens(
                                            body.model or s.model,
                                            len(
                                                context.encoder.encode(
                                                    choice.delta.content
                                                )
                                            ),
                                        )
                                    if choice.delta.refusal is not None:
                                        context.increment_completion_tokens(
                                            body.model or s.model,
                                            len(
                                                context.encoder.encode(
                                                    choice.delta.refusal
                                                )
                                            ),
                                        )
                                yield r.json()
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

                    tasks = [
                        wrapper(index, response)
                        for index, response in enumerate(responses)
                    ]
                    queue = {
                        asyncio.create_task(task.__anext__()): task for task in tasks
                    }

                    while queue:
                        done, _ = await asyncio.wait(
                            queue.keys(), return_when=asyncio.FIRST_COMPLETED
                        )

                        for future in done:
                            task = queue.pop(future)
                            try:
                                yield {"data": await future}
                                queue[asyncio.create_task(task.__anext__())] = task
                            except StopAsyncIteration:
                                continue

                    yield {"data": "[DONE]"}

                    await count_token(context, rate_limiter, rate_limiter_key)

                return sse_starlette.sse.EventSourceResponse(
                    sse(responses), headers=rate_limit_headers(rate_limit_info)
                )
            else:
                response = None
                for index, r in enumerate(responses):
                    if response is None:
                        response = r
                    else:
                        for choice in r.choices:
                            choice.index = index
                        response.choices.extend(r.choices)
                        response.usage.completion_tokens += r.usage.completion_tokens

                context.increment_completion_tokens(
                    body.model or s.model, response.usage.completion_tokens
                )

                await count_token(context, rate_limiter, rate_limiter_key)

                return fastapi.responses.JSONResponse(
                    response.model_dump(), headers=rate_limit_headers(rate_limit_info)
                )
        else:
            context.increment_prompt_tokens(
                body.model,
                len(
                    context.encoder.encode(
                        "".join(
                            json.dumps(
                                message, ensure_ascii=False, separators=(",", ":")
                            )
                            for message in body.messages
                        )
                    )
                ),
            )

            response = await cortex.llm.openai.AsyncOpenAI(
                http_client=httpx.AsyncClient(
                    timeout=None,
                    mounts={
                        "http://": httpx.AsyncHTTPTransport(
                            proxy=os.getenv("HTTP_PROXY")
                        ),
                        "https://": httpx.AsyncHTTPTransport(
                            proxy=os.getenv("HTTPS_PROXY")
                        ),
                    },
                    verify=os.getenv("SSL_CERT_FILE"),
                ),
            ).chat.completions.create(
                model=body.model.replace(".", "")
                if os.getenv("OPENAI_API_TYPE") == "azure"
                else body.model,
                reasoning_effort=body.reasoning_effort
                if body.reasoning_effort is not None
                else openai._types.NOT_GIVEN,
                messages=[message.model_dump() for message in body.messages],
                n=body.n,
                response_format=body.response_format
                if body.response_format is not None
                else openai._types.NOT_GIVEN,
                stream=body.stream,
            )

            if body.stream:

                async def sse(response) -> collections.abc.AsyncIterable[str | bytes]:
                    try:
                        async for r in response:
                            for choice in r.choices:
                                if choice.delta.content is not None:
                                    context.increment_completion_tokens(
                                        body.model,
                                        len(
                                            context.encoder.encode(choice.delta.content)
                                        ),
                                    )
                                if choice.delta.refusal is not None:
                                    context.increment_completion_tokens(
                                        body.model or s.model,
                                        len(
                                            context.encoder.encode(choice.delta.refusal)
                                        ),
                                    )

                            yield {"data": r.json()}
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
                    yield {"data": "[DONE]"}

                    await count_token(context, rate_limiter, rate_limiter_key)

                return sse_starlette.sse.EventSourceResponse(
                    sse(response), headers=rate_limit_headers(rate_limit_info)
                )
            else:
                context.increment_completion_tokens(
                    body.model, response.usage.completion_tokens
                )

                await count_token(context, rate_limiter, rate_limiter_key)

                return fastapi.responses.JSONResponse(
                    response.model_dump(), headers=rate_limit_headers(rate_limit_info)
                )
    except openai.RateLimitError:
        return fastapi.responses.JSONResponse(
            {"error": {"message": "Too Many Requests", "type": "rate_limit_error"}},
            status_code=429,
        )
    except openai.BadRequestError as e:
        return fastapi.responses.JSONResponse(
            e.response.json(),
            status_code=400,
        )
    except cortex.exceptions.InsufficientBudgetError:
        return fastapi.responses.JSONResponse(
            {"error": {"message": "Bad Request", "type": "invalid_request_error"}},
            status_code=400,
        )


@app.websocket("/v1/realtime")
async def realtime(
    websocket: fastapi.WebSocket,
    model: str = fastapi.Query(default="gpt-4o-realtime-preview-2025-06-03"),
    x_auth_request_user: str = fastapi.Header(default="anonymous"),
):
    context = APIContext(
        context_id=uuid.uuid4().hex,
        memory_type=s.memory_type,
        embedding_model=s.embedding_model,
        encoder=encoder,
        user=x_auth_request_user,
    )

    rate_limiter = await get_rate_limiter()
    rate_limiter_key = x_auth_request_user
    limit = context.limit or s.rate_limit_per_interval
    rate_limit_info = await rate_limiter.remaining(rate_limiter_key, limit)

    if rate_limit_info.is_exceeded:
        return fastapi.responses.JSONResponse(
            {"error": {"message": "Too Many Requests", "type": "rate_limit_error"}},
            status_code=429,
        )

    subprotocols = websocket.scope["subprotocols"]
    if len(subprotocols) > 0:
        await websocket.accept(subprotocol=subprotocols[0])
    else:
        await websocket.accept()

    url = f"wss://api.openai.com/v1/realtime?model={model}"
    async for client in websockets.asyncio.client.connect(
        url,
        additional_headers={
            "Authorization": f"Bearer {os.getenv('OPENAI_API_KEY')}",
            "OpenAI-Beta": "realtime=v1",
        },
    ):
        try:

            async def producer():
                try:
                    async for message in websocket.iter_text():
                        await client.send(message)
                except Exception:
                    pass

            async def consumer():
                try:
                    async for message in client:
                        r = json.loads(message)
                        if r["type"] == "response.done":
                            context.increment_prompt_tokens(
                                model,
                                r["response"]["usage"]["input_tokens"],
                            )
                            context.increment_completion_tokens(
                                model,
                                r["response"]["usage"]["output_tokens"],
                            )

                            await count_token(context, rate_limiter, rate_limiter_key)

                        await websocket.send_text(message)
                except Exception:
                    pass

            consumer_task = asyncio.create_task(consumer())
            producer_task = asyncio.create_task(producer())

            done, pending = await asyncio.wait(
                [consumer_task, producer_task],
                return_when=asyncio.FIRST_COMPLETED,
            )
            for task in pending:
                task.cancel()

            return None
        except websockets.exceptions.ConnectionClosedError:
            continue
    return None


# https://platform.openai.com/docs/api-reference/images/create
class V1ImagesGenerations(pydantic.BaseModel):
    prompt: str = pydantic.Field(
        ...,
        description="A text description of the desired image(s). The maximum length is 1000 characters for dall-e-2 and 4000 characters for dall-e-3.",
    )
    background: typing.Literal["transparent", "opaque", "auto"] | None = pydantic.Field(
        None,
        description="Allows to set transparency for the background of the generated image(s). This parameter is only supported for gpt-image-1. Must be one of transparent, opaque or auto (default value). When auto is used, the model will automatically determine the best background for the image. If transparent, the output format needs to support transparency, so it should be set to either png (default value) or webp.",
    )
    model: typing.Literal["dall-e-2", "dall-e-3", "gpt-image-1"] = pydantic.Field(
        "dall-e-2",
        description="The model to use for image generation.",
    )
    moderation: typing.Literal["low", "auto"] | None = pydantic.Field(
        None,
        description="Control the content-moderation level for images generated by gpt-image-1. Must be either low for less restrictive filtering or auto (default value).",
    )
    n: int = pydantic.Field(
        1,
        description="The number of images to generate. Must be between 1 and 10. For dall-e-3, only n=1 is supported.",
    )
    output_compression: int | None = pydantic.Field(
        None,
        ge=0,
        le=100,
        description="The compression level (0-100%) for the generated images. This parameter is only supported for gpt-image-1 with the webp or jpeg output formats, and defaults to 100.",
    )
    output_format: typing.Literal["png", "jpeg", "webp"] | None = pydantic.Field(
        None,
        description="The format in which the generated images are returned. This parameter is only supported for gpt-image-1. Must be one of png, jpeg, or webp.",
    )
    quality: typing.Literal["standard", "hd", "high", "medium", "low", "auto"] = (
        pydantic.Field(
            "auto",
            description="The quality of the image that will be generated. auto (default value) will automatically select the best quality for the given model. hd and standard are supported for dall-e-3. high, medium and low are supported for gpt-image-1. standard is the only option for dall-e-2.",
        )
    )
    response_format: typing.Literal["url", "b64_json"] | None = pydantic.Field(
        None,
        description="The format in which generated images with dall-e-2 and dall-e-3 are returned. Must be one of url or b64_json. URLs are only valid for 60 minutes after the image has been generated. This parameter isn't supported for gpt-image-1 which will always return base64-encoded images.",
    )
    size: typing.Literal[
        "256x256",
        "512x512",
        "1024x1024",
        "1792x1024",
        "1024x1792",
        "1536x1024",
        "1024x1536",
        "auto",
    ] = pydantic.Field(
        "1024x1024",
        description="The size of the generated images. Must be one of 1024x1024, 1536x1024 (landscape), 1024x1536 (portrait), or auto (default value) for gpt-image-1, one of 256x256, 512x512, or 1024x1024 for dall-e-2, and one of 1024x1024, 1792x1024, or 1024x1792 for dall-e-3.",
    )
    style: typing.Literal["vivid", "natural"] | None = pydantic.Field(
        None,
        description="The style of the generated images. This parameter is only supported for dall-e-3. Must be one of vivid or natural. Vivid causes the model to lean towards generating hyper-real and dramatic images. Natural causes the model to produce more natural, less hyper-real looking images.",
    )


@app.post("/v1/images/generations")
async def generations(
    body: V1ImagesGenerations,
    x_auth_request_user: str = fastapi.Header(default="anonymous"),
):
    context = APIContext(
        context_id=uuid.uuid4().hex,
        memory_type=s.memory_type,
        embedding_model=s.embedding_model,
        encoder=encoder,
        user=x_auth_request_user,
    )

    rate_limiter = await get_rate_limiter()
    rate_limiter_key = x_auth_request_user
    limit = context.limit or s.rate_limit_per_interval

    rate_limit_info = await rate_limiter.remaining(rate_limiter_key, limit)

    if rate_limit_info.is_exceeded:
        retry_after = max(1, rate_limit_info.reset_timestamp - int(time.time()))
        rate_limit_info.retry_after = retry_after

        return fastapi.responses.JSONResponse(
            {"error": {"message": "Too Many Requests", "type": "rate_limit_error"}},
            status_code=429,
            headers=rate_limit_headers(rate_limit_info),
        )

    try:
        response = await cortex.llm.openai.AsyncOpenAI(
            http_client=httpx.AsyncClient(
                timeout=None,
                mounts={
                    "http://": httpx.AsyncHTTPTransport(proxy=os.getenv("HTTP_PROXY")),
                    "https://": httpx.AsyncHTTPTransport(
                        proxy=os.getenv("HTTPS_PROXY")
                    ),
                },
                verify=os.getenv("SSL_CERT_FILE"),
            ),
        ).images.generate(
            prompt=body.prompt,
            background=body.background
            if body.background is not None
            else openai._types.NOT_GIVEN,
            model=body.model,
            moderation=body.moderation
            if body.moderation is not None
            else openai._types.NOT_GIVEN,
            n=body.n,
            output_compression=body.output_compression
            if body.output_compression is not None
            else openai._types.NOT_GIVEN,
            output_format=body.output_format
            if body.output_format is not None
            else openai._types.NOT_GIVEN,
            quality=body.quality,
            response_format=body.response_format
            if body.response_format is not None
            else openai._types.NOT_GIVEN,
            size=body.size,
            style=body.style if body.style is not None else openai._types.NOT_GIVEN,
        )
    except openai.BadRequestError as e:
        return fastapi.responses.JSONResponse(
            e,
            status_code=400,
        )
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

    size = body.size
    quality = body.quality
    if quality == "auto":
        match body.model:
            case "dall-e-2":
                quality = "standard"
            case "dall-e-3":
                quality = "hd"
            case "gpt-image-1":
                quality = "high"

    if body.model == "dall-e-2":
        image_model = cortex.llm.openai.model.ImageModel(f"{size}")
    else:
        image_model = cortex.llm.openai.model.ImageModel(f"{quality}-{size}")

    get_generated_images_total().add(
        body.n,
        attributes={
            "model": image_model,
            "user": context.user,
        },
    )

    if body.model == "gpt-image-1":
        context.increment_prompt_tokens(
            cortex.llm.openai.model.CompletionModel.GPT_IMAGE_1,
            response.usage["input_tokens"],
        )
        context.increment_completion_tokens(
            cortex.llm.openai.model.CompletionModel.GPT_IMAGE_1,
            response.usage["output_tokens"],
        )

        await count_token(context, rate_limiter, rate_limiter_key)

    await rate_limiter.take(
        rate_limiter_key,
        int((image_model.price / s.model.prices["price_per_prompt"]) * body.n),
    )

    return fastapi.responses.JSONResponse(
        response.model_dump(), headers=rate_limit_headers(rate_limit_info)
    )


@app.exception_handler(404)
async def not_found_exception_handler(
    request: fastapi.Request,
    _exc: fastapi.exceptions.HTTPException,
):
    client = httpx.AsyncClient(
        timeout=None,
        mounts={
            "http://": httpx.AsyncHTTPTransport(proxy=os.getenv("HTTP_PROXY")),
            "https://": httpx.AsyncHTTPTransport(proxy=os.getenv("HTTPS_PROXY")),
        },
        verify=os.getenv("SSL_CERT_FILE"),
    )

    headers = {
        "Authorization": f"Bearer {os.getenv('OPENAI_API_KEY')}",
    }
    if (content_type := request.headers.get("content-type")) is not None:
        headers["Content-Type"] = content_type

    request = client.build_request(
        request.method,
        f"https://api.openai.com{request.url.path}",
        content=await request.body(),
        headers=headers,
    )
    response = await client.send(request, stream=True)

    async def streaming():
        try:
            async for chunk in response.aiter_raw():
                yield chunk
        finally:
            await response.aclose()
            await client.aclose()

    return fastapi.responses.StreamingResponse(
        streaming(),
        status_code=response.status_code,
        headers={k: v for k, v in response.headers.items()},
        media_type=response.headers.get("content-type"),
    )


@app.get("/healthz")
def health() -> str:
    return "OK"


@app.get("/metrics")
def metrics() -> fastapi.Response:
    return fastapi.Response(
        prometheus_client.generate_latest(),
        media_type=prometheus_client.CONTENT_TYPE_LATEST,
    )


if __name__ == "__main__":
    if s.is_debug():
        import dotenv

        dotenv.load_dotenv(override=True)

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
