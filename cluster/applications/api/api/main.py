import asyncio
import collections.abc
import datetime
import json
import logging
import os
import typing
import uuid

import aredis_om
import dotenv
import fastapi
import google.oauth2.credentials
import httpx
import openai
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

import api.agent.root_agent
import api.settings
import api.telemetry
import cortex.exceptions
import cortex.llm.openai.agent.memory
import cortex.llm.openai.model
import cortex.rate_limit

s = api.settings.Settings()
app = fastapi.FastAPI()
opentelemetry.instrumentation.fastapi.FastAPIInstrumentor.instrument_app(app)

encoder: tiktoken.Encoding = tiktoken.get_encoding("cl100k_base")


@app.on_event("startup")
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
    opentelemetry.instrumentation.requests.RequestsInstrumentor().instrument()
    opentelemetry.instrumentation.httpx.HTTPXClientInstrumentor().instrument()
    opentelemetry.instrumentation.redis.RedisInstrumentor().instrument()

    opentelemetry.metrics.set_meter_provider(opentelemetry.sdk.metrics.MeterProvider(
        metric_readers=[opentelemetry.exporter.prometheus.PrometheusMetricReader()],
    ))

    with api.telemetry.tracer.start_as_current_span(
        "startup",
    ):
        match s.memory_type:
            case cortex.llm.openai.agent.MemoryType.Redis:
                cortex.llm.openai.agent.memory.RedisMemory.Meta.database = redis.asyncio.Redis(
                    host=s.redis_host,
                    port=s.redis_port,
                    retry=redis.asyncio.retry.Retry(redis.backoff.ExponentialBackoff(), 3),
                    retry_on_error=[redis.exceptions.ConnectionError, redis.exceptions.ReadOnlyError],
                )
                await aredis_om.Migrator().run()


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
        api.telemetry.logger.error(e, exc_info=e)
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


global_rate_limiter: cortex.rate_limit.RateLimiter | None = None


async def get_rate_limiter() -> cortex.rate_limit.RateLimiter:
    global global_rate_limiter
    if global_rate_limiter is None:
        match s.rate_limiter_type:
            case cortex.llm.openai.agent.MemoryType.Redis:
                global_rate_limiter = cortex.rate_limit.RedisSlidingRateLimiter(redis.asyncio.Redis(
                    host=s.redis_host,
                    port=s.redis_port,
                    retry=redis.asyncio.retry.Retry(redis.backoff.ExponentialBackoff(), 3),
                    retry_on_error=[redis.exceptions.ConnectionError, redis.exceptions.ReadOnlyError],
                ), limit=s.rate_limit_per_interval, interval_seconds=s.rate_limit_interval_seconds)
            case _:
                raise NotImplementedError
    return global_rate_limiter


class Message(pydantic.BaseModel):
    role: str
    content: str


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

    async def report_progress(self, message: str, stage: cortex.llm.openai.agent.ProgressStage):
        pass


async def count_token(context: APIContext, rate_limiter: cortex.rate_limit.RateLimiter, rate_limiter_key: str):
    prompt_tokens = context.prompt_tokens
    completion_tokens = context.completion_tokens
    embedding_tokens = context.embedding_tokens

    sum_prompt_tokens = sum(prompt_tokens.values())
    sum_completion_tokens = sum(completion_tokens.values())
    sum_embedding_tokens = sum(embedding_tokens.values())
    await rate_limiter.take(rate_limiter_key, sum_prompt_tokens + sum_completion_tokens)

    api.telemetry.logger.info(
        json.dumps({
            "prompt_tokens": sum_prompt_tokens,
            "completion_tokens": sum_completion_tokens,
            "embedding_tokens": sum_embedding_tokens,
            "key": rate_limiter_key,
            "user": context.user,
        }),
    )

    for model, tokens in prompt_tokens.items():
        get_processed_tokens_total().add(
            tokens,
            attributes={
                "usage": "prompt",
                "model": model,
                "key": rate_limiter_key,
                "user": context.user,
            },
        )
    for model, tokens in completion_tokens.items():
        get_processed_tokens_total().add(
            tokens,
            attributes={
                "usage": "completion",
                "model": model,
                "key": rate_limiter_key,
                "user": context.user,
            },
        )
    for model, tokens in embedding_tokens.items():
        get_processed_tokens_total().add(
            tokens,
            attributes={
                "usage": "embedding",
                "model": model,
                "key": rate_limiter_key,
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
    stream: bool = pydantic.Field(
        False,
        description="If set, partial message deltas will be sent, like in ChatGPT. Tokens will be sent as data-only [server-sent events](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events/Using_server-sent_events#Event_stream_format) as they become available, with the stream terminated by a data: [DONE] message. [Example Python code](https://cookbook.openai.com/examples/how_to_stream_completions).",
    )
    n: int = pydantic.Field(
        1,
        description="How many chat completion choices to generate for each input message. Note that you will be charged based on the number of generated tokens across all of the choices. Keep n as 1 to minimize costs.",
    )


@app.post("/v1/chat/completions")
async def completions(body: V1ChatCompletions, x_auth_request_user: str = fastapi.Header(default="anonymous")):
    rate_limiter = await get_rate_limiter()
    rate_limiter_key = x_auth_request_user
    if not await rate_limiter.remaining(rate_limiter_key):
        return fastapi.responses.JSONResponse(
            {"error": {"message": "Too Many Requests", "type": "rate_limit_error"}},
            status_code=429,
        )

    instructions = [
        "Your task is to deliver a concise and accurate response to a user's query."
        "Your answer must be precise, of high-quality, and written by an expert."
        "It is EXTREMELY IMPORTANT to directly answer the query."
        "NEVER say \"based on the search results\" or start your answer with a heading or title."
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
        f"Now: {datetime.datetime.now().isoformat()}\n"
    ]
    if s.system_prompt is not None:
        instructions.append(s.system_prompt)

    messages = []
    if len(instructions) > 0:
        system_message = {"role": "system", "content": "\n".join(instructions)}
        messages.append(system_message)

    for message in body.messages:
        messages.append(message.dict())

    if len(messages) == 0:
        return

    context = APIContext(
        context_id=uuid.uuid4().hex,
        memory_type=s.memory_type,
        embedding_model=s.embedding_model,
        encoder=encoder,
        user=x_auth_request_user,
    )
    await context.acquire_budget(s.loop_budget)

    try:
        responses = await asyncio.gather(*[
            api.agent.root_agent.RootAgent(
                embedding_retrieval_endpoint=s.embedding_retrieval_endpoint,
                github_token=s.github_token,
                slack_token=s.slack_bot_token,
                google_credentials=google.oauth2.credentials.Credentials.from_authorized_user_info({
                    "client_id": s.google_client_id,
                    "client_secret": s.google_client_secret,
                    "refresh_token": s.google_pre_issued_refresh_token,
                    "token_uri": "https://accounts.google.com/o/oauth2/token",
                    "scopes": cortex.GOOGLE_OAUTH_SCOPES,
                }),
                bing_subscription_key=s.bing_subscription_key,
                model=body.model or s.model,
                encoder=encoder,
            ).chat_completion_loop(
                messages,
                context,
                streaming=body.stream,
            )
            for _ in range(body.n)
        ])
    except openai.RateLimitError:
        return fastapi.responses.JSONResponse(
            {"error": {"message": "Too Many Requests", "type": "rate_limit_error"}},
            status_code=429,
        )
    except openai.BadRequestError as e:
        return fastapi.responses.JSONResponse(
            e,
            status_code=400,
        )
    except cortex.exceptions.InsufficientBudgetError:
        return fastapi.responses.JSONResponse(
            {"error": {"message": "Bad Request", "type": "invalid_request_error"}},
            status_code=400,
        )

    if body.stream:
        async def sse(responses) -> collections.abc.AsyncIterable[str | bytes]:
            async def wrapper(index, response):
                try:
                    async for r in response:
                        for choice in r.choices:
                            choice.index = index
                            if choice.delta.content is not None:
                                context.increment_completion_tokens(
                                    body.model or s.model,
                                    len(encoder.encode(choice.delta.content)),
                                )
                        yield r.json()
                except (httpx.RemoteProtocolError, httpx.ReadTimeout, openai.APIConnectionError) as e:
                    raise cortex.exceptions.RetryableError(e) from e
                except openai.APIError as e:
                    if e.code is not None and e.code == "server_error":
                        raise cortex.exceptions.RetryableError(e) from e
                    if e.code == "rate_limit_exceeded":
                        raise cortex.exceptions.RetryableError(e) from e
                    raise e

            tasks = [wrapper(index, response) for index, response in enumerate(responses)]
            queue = {asyncio.create_task(task.__anext__()): task for task in tasks}

            while queue:
                done, _ = await asyncio.wait(queue.keys(), return_when=asyncio.FIRST_COMPLETED)

                for future in done:
                    task = queue.pop(future)
                    try:
                        yield {"data": await future}
                        queue[asyncio.create_task(task.__anext__())] = task
                    except StopAsyncIteration:
                        continue

            yield {"data": "[DONE]"}

            await count_token(context, rate_limiter, rate_limiter_key)

        return sse_starlette.sse.EventSourceResponse(sse(responses))
    else:
        response = None
        for index, r in enumerate(responses):
            if response is None:
                response = r
            else:
                for choice in r.choices:
                    choice.index = index
                response.choices.extend(r.choices)

            context.increment_completion_tokens(body.model or s.model, r.usage.completion_tokens)

        await count_token(context, rate_limiter, rate_limiter_key)

        return fastapi.responses.JSONResponse(response.dict())


# https://platform.openai.com/docs/api-reference/images/create
class V1ImagesGenerations(pydantic.BaseModel):
    prompt: str = pydantic.Field(
        ...,
        description="A text description of the desired image(s). The maximum length is 1000 characters for dall-e-2 and 4000 characters for dall-e-3.",
    )
    model: typing.Literal["dall-e-2", "dall-e-3"] = pydantic.Field(
        "dall-e-2",
        description="The model to use for image generation.",
    )
    n: int = pydantic.Field(
        1,
        description="The number of images to generate. Must be between 1 and 10. For dall-e-3, only n=1 is supported.",
    )
    quality: typing.Literal["standard", "hd"] = pydantic.Field(
        "standard",
        description="The quality of the image that will be generated. hd creates images with finer details and greater consistency across the image. This param is only supported for dall-e-3.",
    )
    response_format: typing.Literal["url", "b64_json"] = pydantic.Field(
        "url",
        description="The format in which the generated images are returned. Must be one of url or b64_json. URLs are only valid for 60 minutes after the image has been generated.",
    )
    size: typing.Literal["256x256", "512x512", "1024x1024", "1792x1024", "1024x1792"] = pydantic.Field(
        "1024x1024",
        description="The size of the generated images. Must be one of 256x256, 512x512, or 1024x1024 for dall-e-2. Must be one of 1024x1024, 1792x1024, or 1024x1792 for dall-e-3 models.",
    )
    style: typing.Literal["vivid", "natural"] = pydantic.Field(
        "vivid",
        description="The style of the generated images. Must be one of vivid or natural. Vivid causes the model to lean towards generating hyper-real and dramatic images. Natural causes the model to produce more natural, less hyper-real looking images. This param is only supported for dall-e-3.",
    )


@app.post("/v1/images/generations")
async def generations(body: V1ImagesGenerations, x_auth_request_user: str = fastapi.Header(default="anonymous")):
    rate_limiter = await get_rate_limiter()
    rate_limiter_key = x_auth_request_user
    if not await rate_limiter.remaining(rate_limiter_key):
        return fastapi.responses.JSONResponse(
            {"error": {"message": "Too Many Requests", "type": "rate_limit_error"}},
            status_code=429,
        )

    context = APIContext(
        context_id=uuid.uuid4().hex,
        memory_type=s.memory_type,
        embedding_model=s.embedding_model,
        encoder=encoder,
        user=x_auth_request_user,
    )

    try:
        response = await cortex.llm.openai.AsyncOpenAI(
            http_client=httpx.AsyncClient(proxies={
                "http://": os.getenv("HTTP_PROXY"),
                "https://": os.getenv("HTTPS_PROXY"),
            }, verify=os.getenv("SSL_CERT_FILE")),
        ).images.generate(
            prompt=body.prompt,
            model=body.model,
            n=body.n,
            quality=body.quality,
            response_format=body.response_format,
            size=body.size,
            style=body.style,
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
            case 409 | 429 | 502 | 503 | 504:
                raise cortex.exceptions.RetryableError(e) from e
        raise e

    if body.model == "dall-e-2":
        image_model = cortex.llm.openai.model.ImageModel(f"{body.size}")
    else:
        image_model = cortex.llm.openai.model.ImageModel(f"{body.quality}-{body.size}")

    context.increment_generated_images(image_model, body.n)

    await rate_limiter.take(rate_limiter_key, int((image_model.price / s.model.prices["price_per_prompt"]) * body.n))

    return response


@app.get("/health")
def health() -> str:
    return "OK"


@app.get("/metrics")
def metrics() -> fastapi.Response:
    return fastapi.Response(prometheus_client.generate_latest(), media_type=prometheus_client.CONTENT_TYPE_LATEST)


if __name__ == "__main__":
    import uvicorn

    uvicorn.run(
        "main:app",
        host=s.host,
        port=s.port,
        log_level=s.convert_log_level(),
        timeout_keep_alive=s.idle_timeout,
        timeout_graceful_shutdown=s.termination_grace_period_seconds,
        reload=s.reload,
        access_log=s.access_log,
    )
