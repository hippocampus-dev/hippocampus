import contextlib
import datetime
import http
import logging
import typing

import fastapi
import fastapi_mcp
import opentelemetry.exporter.otlp.proto.grpc.trace_exporter
import opentelemetry.exporter.prometheus
import opentelemetry.instrumentation.fastapi
import opentelemetry.instrumentation.httpx
import opentelemetry.metrics
import opentelemetry.sdk.metrics
import opentelemetry.sdk.resources
import opentelemetry.sdk.trace.export
import opentelemetry.trace
import prometheus_client
import pythonjsonlogger.jsonlogger
import tiktoken

import embedding_retrieval.datastore
import embedding_retrieval.exceptions
import embedding_retrieval.model
import embedding_retrieval.settings
import embedding_retrieval.telemetry

s = embedding_retrieval.settings.Settings()


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
    logging.basicConfig(level=s.convert_log_level(), handlers=[handler])

    provider = opentelemetry.sdk.trace.TracerProvider(
        resource=opentelemetry.sdk.resources.OTELResourceDetector().detect(),
    )
    processor = opentelemetry.sdk.trace.export.BatchSpanProcessor(
        opentelemetry.exporter.otlp.proto.grpc.trace_exporter.OTLPSpanExporter(),
    )
    provider.add_span_processor(processor)
    opentelemetry.trace.set_tracer_provider(provider)

    opentelemetry.instrumentation.httpx.HTTPXClientInstrumentor().instrument()

    opentelemetry.metrics.set_meter_provider(
        opentelemetry.sdk.metrics.MeterProvider(
            metric_readers=[opentelemetry.exporter.prometheus.PrometheusMetricReader()],
        )
    )


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
    except embedding_retrieval.exceptions.RetryableError as e:
        embedding_retrieval.telemetry.logger.error(e)
        return fastapi.responses.PlainTextResponse(
            http.HTTPStatus.SERVICE_UNAVAILABLE.phrase,
            status_code=http.HTTPStatus.SERVICE_UNAVAILABLE,
        )
    except Exception as e:
        embedding_retrieval.telemetry.logger.error(e, exc_info=True)
        return fastapi.responses.PlainTextResponse(
            http.HTTPStatus.INTERNAL_SERVER_ERROR.phrase,
            status_code=http.HTTPStatus.INTERNAL_SERVER_ERROR,
        )


global_datastore: embedding_retrieval.datastore.DataStore | None = None


async def get_datastore() -> embedding_retrieval.datastore.DataStore:
    global global_datastore

    if not global_datastore:
        e = s.datastore
        match e:
            case embedding_retrieval.settings.DataStore.Qdrant:
                from embedding_retrieval.datastore.qdrant import QdrantDataStore

                global_datastore = QdrantDataStore(
                    s.default_chunk_size,
                    s.embedding_batch_size,
                    s.embedding_model,
                    tiktoken.get_encoding("cl100k_base"),
                    s.qdrant_host,
                    s.qdrant_port,
                    s.qdrant_timeout,
                    s.qdrant_collection_name,
                    s.qdrant_replication_factor,
                    s.qdrant_shard_number,
                )
            case _:
                raise ValueError(f"Unsupported vector database: {e}")
    await global_datastore.init()
    return global_datastore


@app.post("/upsert", operation_id="upsert_documents")
async def upsert(
    body: embedding_retrieval.model.UpsertRequest,
    d: embedding_retrieval.datastore.DataStore = fastapi.Depends(get_datastore),
) -> embedding_retrieval.model.UpsertResponse:
    ids = await d.upsert(body.documents)
    return embedding_retrieval.model.UpsertResponse(ids=ids)


@app.post("/query", operation_id="query_documents")
async def query(
    body: embedding_retrieval.model.QueryRequest,
    d: embedding_retrieval.datastore.DataStore = fastapi.Depends(get_datastore),
) -> embedding_retrieval.model.QueryResponse:
    results = await d.query(body.queries)
    return embedding_retrieval.model.QueryResponse(results=results)


@app.delete("/delete", operation_id="delete_documents")
async def delete(
    body: embedding_retrieval.model.DeleteRequest,
    d: embedding_retrieval.datastore.DataStore = fastapi.Depends(get_datastore),
) -> embedding_retrieval.model.DeleteResponse:
    success = await d.delete(body.filter)
    return embedding_retrieval.model.DeleteResponse(success=success)


mcp = fastapi_mcp.FastApiMCP(app)
mcp.mount(mount_path="/sse")


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
