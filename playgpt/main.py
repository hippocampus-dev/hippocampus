import asyncio
import collections.abc
import contextlib
import datetime
import logging
import typing

import fastapi
import opentelemetry.exporter.otlp.proto.grpc.trace_exporter
import opentelemetry.instrumentation.fastapi
import opentelemetry.sdk.resources
import opentelemetry.sdk.trace.export
import opentelemetry.trace
import playwright.async_api
import pydantic
import pythonjsonlogger.jsonlogger
import tiktoken

import playgpt.settings

s = playgpt.settings.Settings()


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
        handler.setFormatter(
            logging.Formatter(
                "time=%(asctime)s name=%(name)s severitytext=%(levelname)s body=%(message)s traceid=%(traceid)s spanid=%(spanid)s",
                defaults={"traceid": "", "spanid": ""},
            )
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


@contextlib.asynccontextmanager
async def lifespan(_app: fastapi.FastAPI):
    await startup()
    yield
    if playwright_driver is not None:
        await playwright_driver.stop()


class Message(pydantic.BaseModel):
    role: str
    content: str


class ChatCompletionsRequest(pydantic.BaseModel):
    model: str
    messages: collections.abc.Sequence[Message]


class Usage(pydantic.BaseModel):
    prompt_tokens: int
    completion_tokens: int
    total_tokens: int


class Choice(pydantic.BaseModel):
    message: Message
    finish_reason: str = "stop"
    index: int = 0


class ChatCompletionsResponse(pydantic.BaseModel):
    id: str | None = None
    object: str = "chat.completion"
    created: int
    model: str
    usage: Usage
    choices: collections.abc.Sequence[Choice]


encoder: tiktoken.Encoding = tiktoken.get_encoding("cl100k_base")

app = fastapi.FastAPI(lifespan=lifespan)
opentelemetry.instrumentation.fastapi.FastAPIInstrumentor.instrument_app(app)

page: playwright.async_api.Page | None = None
playwright_driver: playwright.async_api.Playwright | None = None


async def get_page() -> playwright.async_api.Page:
    global page, playwright_driver
    if page is None or page.is_closed():
        playwright_driver = await playwright.async_api.async_playwright().start()
        browser = await playwright_driver.chromium.connect_over_cdp(
            "http://127.0.0.1:59222"
        )
        context = await browser.new_context()
        await context.grant_permissions(["clipboard-read", "clipboard-write"])

        await context.add_cookies(
            [
                {
                    "name": "__Secure-next-auth.session-token",
                    "value": s.openai_session_token,
                    "domain": "chat.openai.com",
                    "path": "/",
                    "httpOnly": True,
                    "secure": True,
                    "sameSite": "Lax",
                }
            ]
        )

        page = await context.new_page()
        await page.set_viewport_size({"width": 1920, "height": 1080})
        await page.goto("https://chat.openai.com/chat", wait_until="networkidle")

        try:
            if element := await asyncio.wait_for(
                page.wait_for_selector(
                    "css=div#challenge-stage >> css=div >> css=label >> css=input"
                ),
                timeout=1,
            ):
                await element.click()
        except asyncio.TimeoutError:
            pass

        if element := await page.wait_for_selector("*css=button >> text=Next"):
            await element.click()

        if element := await page.wait_for_selector("*css=button >> text=Next"):
            await element.click()

        if element := await page.wait_for_selector("*css=button >> text=Done"):
            await element.click()

    return page


@app.post("/v1/chat/completions")
async def completion(
    body: ChatCompletionsRequest,
    p: playwright.async_api.Page = fastapi.Depends(get_page),
) -> ChatCompletionsResponse:
    if element := await p.wait_for_selector("*css=a >> text=New chat"):
        await element.click()

    if element := await p.wait_for_selector("*css=button >> text=Model"):
        await element.click()

    try:
        model = get_model(body.model)
    except ValueError:
        raise fastapi.HTTPException(status_code=400, detail="Invalid model")

    if element := await p.wait_for_selector(f"*css=li >> text={model}"):
        await element.click()

    start_end = asyncio.Queue(maxsize=2 * len(body.messages))

    async def handle_response(response):
        # `Moderations` is called twice at the beginning and end of the request
        if response.request.url == "https://chat.openai.com/backend-api/moderations":
            await start_end.put(None)

    p.on("response", handle_response)

    prompt_tokens = 0
    for message in body.messages:
        prompt_tokens += len(encoder.encode(message.content))

        if message.role == "assistant":
            continue

        if element := await p.wait_for_selector("*css=textarea"):
            await element.fill(message.content)

        if element := await p.wait_for_selector(
            '*css=button >> css=svg >> css=polygon[points="22 2 15 22 11 13 2 9 22 2"]',
        ):
            await element.click()

        if message.role == "system":
            if element := await p.wait_for_selector(
                "*css=button >> text=Stop generating"
            ):
                await element.click()

            await start_end.get()
        else:
            await start_end.get()
            await start_end.get()

    groups = await p.query_selector_all("div.group")
    last_group = groups.pop()
    if buttons := await last_group.query_selector_all(
        '*css=button >> css=svg >> css=path[d="M16 4h2a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h2"]',
    ):
        last_button = buttons.pop()
        await last_button.click()
    text = await p.evaluate("navigator.clipboard.readText()")

    completion_tokens = len(encoder.encode(text))
    return ChatCompletionsResponse(
        created=datetime.datetime.now(datetime.timezone.utc).timestamp(),
        model=body.model,
        usage=Usage(
            prompt_tokens=prompt_tokens,
            completion_tokens=completion_tokens,
            total_tokens=prompt_tokens + completion_tokens,
        ),
        choices=[
            Choice(
                message=Message(
                    role="assistant",
                    content=text,
                ),
            )
        ],
    )


# https://platform.openai.com/docs/models/model-endpoint-compatibility
def get_model(model: str) -> str:
    match model:
        case "gpt-4" | "gpt-4-32k":
            return "GPT-4"
        case "gpt-3.5-turbo" | "gpt-3.5-turbo-16k":
            return "Default (GPT-3.5)"
        case _:
            raise ValueError(f"Unknown model: {model}")


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
    )
