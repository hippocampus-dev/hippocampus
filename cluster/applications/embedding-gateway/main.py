import asyncio
import collections.abc
import contextlib
import datetime
import gzip
import hashlib
import io
import json
import logging
import os
import typing
import urllib.parse

import aiohttp
import aiohttp.client_exceptions
import boto3
import botocore.exceptions
import fastapi
import opentelemetry.context
import opentelemetry.exporter.otlp.proto.grpc.trace_exporter
import opentelemetry.exporter.prometheus
import opentelemetry.instrumentation.aiohttp_client
import opentelemetry.instrumentation.botocore
import opentelemetry.instrumentation.fastapi
import opentelemetry.metrics
import opentelemetry.sdk.metrics
import opentelemetry.sdk.resources
import opentelemetry.sdk.trace.export
import opentelemetry.trace
import prometheus_client
import pydantic
import pythonjsonlogger.jsonlogger

import embedding_gateway.exceptions
import embedding_gateway.settings
import embedding_gateway.telemetry

s = embedding_gateway.settings.Settings()


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

    opentelemetry.metrics.set_meter_provider(opentelemetry.sdk.metrics.MeterProvider(
        metric_readers=[opentelemetry.exporter.prometheus.PrometheusMetricReader()],
    ))


@contextlib.asynccontextmanager
async def lifespan(_app: fastapi.FastAPI):
    await startup()
    yield


app = fastapi.FastAPI(lifespan=lifespan)
opentelemetry.instrumentation.fastapi.FastAPIInstrumentor.instrument_app(app)


@app.middleware("http")
async def override_server_error_middleware(request: fastapi.Request, call_next):
    try:
        return await call_next(request)
    except embedding_gateway.exceptions.RetryableError as e:
        embedding_gateway.telemetry.logger.error(e)
        return fastapi.responses.PlainTextResponse("Service Unavailable", status_code=503)
    except Exception as e:
        embedding_gateway.telemetry.logger.error(e, exc_info=True)
        return fastapi.responses.PlainTextResponse("Internal Server Error", status_code=500)


global_s3_client: typing.Any = None


def get_s3_client() -> typing.Any:
    global global_s3_client
    if global_s3_client is None:
        global_s3_client = boto3.client("s3", endpoint_url=s.s3_endpoint_url)
    return global_s3_client


global_processed_tokens_total: opentelemetry.metrics.Counter | None = None


def get_processed_tokens_total() -> opentelemetry.metrics.Counter:
    global global_processed_tokens_total
    if global_processed_tokens_total is None:
        global_processed_tokens_total = embedding_gateway.telemetry.meter.create_counter(
            "processed_tokens_total",
            description="Total number of tokens processed",
        )
    return global_processed_tokens_total


class AnyRequest(pydantic.RootModel[collections.abc.Mapping[str, typing.Any]]):
    pass


@app.post("/v1/embeddings")
async def embeddings(
    body: AnyRequest,
    request: fastapi.Request,
    s3_client: typing.Any = fastapi.Depends(get_s3_client),
    processed_tokens_total: opentelemetry.metrics.Counter = fastapi.Depends(get_processed_tokens_total),
) -> typing.Any:
    raw_body = body.root

    hasher = hashlib.sha256()
    hasher.update(json.dumps(raw_body, sort_keys=True).encode("utf-8"))
    digest = hasher.hexdigest()
    key = f"{digest[:4]}/{digest}"

    loop = asyncio.get_running_loop()
    try:
        try:
            def run_in_context(ctx: opentelemetry.context.Context):
                opentelemetry.context.attach(ctx)
                return s3_client.get_object(
                    Bucket=s.s3_bucket,
                    Key=key,
                )

            response = await loop.run_in_executor(None, run_in_context, opentelemetry.context.get_current())
            return json.loads(gzip.decompress(response["Body"].read()).decode("utf-8"))
        except s3_client.exceptions.NoSuchKey:
            with embedding_gateway.telemetry.tracer.start_as_current_span("create_openai_embeddings"):
                e = await create_openai_embeddings(raw_body, dict(request.headers))

            if "error" in e:
                return e

            processed_tokens_total.add(e["usage"]["total_tokens"])

            gzipped = io.BytesIO()
            with gzip.GzipFile(fileobj=gzipped, mode="w") as f:
                f.write(json.dumps(e).encode("utf-8"))

            def run_in_context(ctx: opentelemetry.context.Context):
                opentelemetry.context.attach(ctx)
                return s3_client.put_object(
                    Bucket=s.s3_bucket,
                    Key=key,
                    Body=gzipped.getvalue(),
                    ContentEncoding="gzip",
                    ContentType="application/json",
                )

            await loop.run_in_executor(None, run_in_context, opentelemetry.context.get_current())

            return e
    except botocore.exceptions.ClientError as e:
        match e.response["ResponseMetadata"]["HTTPStatusCode"]:
            case 409 | 429 | 502 | 503 | 504:
                raise embedding_gateway.exceptions.RetryableError(e) from e
        raise e
    except (
            botocore.exceptions.EndpointConnectionError,  # ECONNREFUSED
            botocore.exceptions.ConnectTimeoutError,
            botocore.exceptions.ReadTimeoutError,
            botocore.exceptions.ConnectionClosedError,  # ECONNRESET
    ) as e:
        raise embedding_gateway.exceptions.RetryableError(e) from e


async def create_openai_embeddings(
    body: collections.abc.Mapping[str, typing.Any],
    downstream_headers: dict[str, str],
) -> collections.abc.Mapping[str, typing.Any]:
    base_url = os.getenv("OPENAI_BASE_URL", "https://api.openai.com/v1")

    headers = downstream_headers.copy()
    headers.update({"host": urllib.parse.urlparse(base_url).netloc})
    del headers["content-length"]

    extra_headers = {}
    for k, v in extra_headers.items():
        headers.setdefault(k.lower(), v)

    try:
        async with aiohttp.ClientSession(headers=headers, trust_env=True) as session:
            async with session.post(f"{base_url}/embeddings", json=body) as response:
                if response.status != 200:
                    raise fastapi.HTTPException(response.status)
                with embedding_gateway.telemetry.tracer.start_as_current_span("response.json()"):
                    return await response.json()
    except (
            aiohttp.ClientConnectionError,  # ECONNREFUSED, EPIPE, ECONNRESET
            aiohttp.client_exceptions.ServerDisconnectedError,
            asyncio.TimeoutError,
    ) as e:
        raise embedding_gateway.exceptions.RetryableError(e) from e


@app.get("/healthz")
def health() -> str:
    return "OK"


@app.get("/metrics")
def metrics() -> fastapi.Response:
    return fastapi.Response(prometheus_client.generate_latest(), media_type=prometheus_client.CONTENT_TYPE_LATEST)


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
