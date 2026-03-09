import abc
import asyncio
import collections.abc
import uuid

import fastembed
import httpx
import memory_bank.categorizer
import memory_bank.datastore
import memory_bank.exceptions
import memory_bank.merger
import memory_bank.model
import memory_bank.settings
import memory_bank.telemetry
import numpy
import opentelemetry.context
import qdrant_client
import qdrant_client.http.exceptions
import qdrant_client.http.models
import tiktoken

bm25 = fastembed.SparseTextEmbedding("Qdrant/bm25")
bm42 = fastembed.SparseTextEmbedding("Qdrant/bm42-all-minilm-l6-v2-attentions")


def _convert_metadata_filter_to_qdrant_filter(
    metadata_filter: memory_bank.model.MemoryMetadataFilter,
) -> qdrant_client.http.models.Filter:
    must_conditions = []

    for key, value in metadata_filter.model_dump().items():
        if value is not None:
            must_conditions.append(
                qdrant_client.http.models.FieldCondition(
                    key=f"metadata.{key}",
                    match=qdrant_client.http.models.MatchValue(value=value),
                ),
            )

    return qdrant_client.http.models.Filter(must=must_conditions, should=[])


def _convert_ndarray_tolist(
    m: collections.abc.Mapping[str, numpy.ndarray],
) -> collections.abc.Mapping[str, collections.abc.Sequence]:
    return {k: v.tolist() for k, v in m.items()}


class QdrantDataStore(memory_bank.datastore.DataStore, abc.ABC):
    UUID_NAMESPACE = uuid.UUID("3896d314-1e95-4a3a-b45a-945f9f0b541d")

    def __init__(
        self,
        default_chunk_size: int,
        embedding_batch_size: int,
        embedding_model: memory_bank.settings.EmbeddingModel,
        encoder: tiktoken.Encoding,
        categorizer: memory_bank.categorizer.Categorizer,
        merger: memory_bank.merger.Merger,
        host: str,
        port: int,
        timeout: int,
        collection_name: str,
        replication_factor: int,
        shard_number: int,
    ):
        super().__init__(
            default_chunk_size,
            embedding_batch_size,
            embedding_model,
            encoder,
            categorizer,
            merger,
        )

        self.client = qdrant_client.QdrantClient(
            host=host,
            port=port,
            timeout=timeout,
        )
        self.collection_name = collection_name
        self.replication_factor = replication_factor
        self.shard_number = shard_number

    async def init(self):
        try:
            self.client.get_collection(self.collection_name)
        except qdrant_client.http.exceptions.UnexpectedResponse as e:
            match e.status_code:
                case 404:
                    self.client.create_collection(
                        self.collection_name,
                        vectors_config={
                            "openai": qdrant_client.http.models.VectorParams(
                                size=memory_bank.settings.OPENAI_VECTOR_SIZE,
                                distance=qdrant_client.http.models.Distance.COSINE,
                            ),
                        },
                        sparse_vectors_config={
                            "bm25": qdrant_client.http.models.SparseVectorParams(
                                index=qdrant_client.http.models.SparseIndexParams(
                                    on_disk=False,
                                ),
                                modifier=qdrant_client.http.models.Modifier.IDF,
                            ),
                            "bm42": qdrant_client.http.models.SparseVectorParams(
                                index=qdrant_client.http.models.SparseIndexParams(
                                    on_disk=False,
                                ),
                                modifier=qdrant_client.http.models.Modifier.IDF,
                            ),
                        },
                        replication_factor=self.replication_factor,
                        shard_number=self.shard_number,
                    )

                    self.client.create_payload_index(
                        self.collection_name,
                        field_name="metadata.category",
                        field_type=qdrant_client.http.models.PayloadSchemaType.KEYWORD,
                    )
                    self.client.create_payload_index(
                        self.collection_name,
                        field_name="metadata.memory_id",
                        field_type=qdrant_client.http.models.PayloadSchemaType.KEYWORD,
                    )
                case 409 | 429 | 502 | 503 | 504:
                    raise memory_bank.exceptions.RetryableError(e) from e
                case _:
                    raise e

    async def _get_existing_memory(self, memory_id: str) -> str | None:
        def run_in_context(ctx: opentelemetry.context.Context):
            opentelemetry.context.attach(ctx)
            with memory_bank.telemetry.tracer.start_as_current_span(
                "qdrant.get_existing_memory"
            ):
                return self.client.scroll(
                    collection_name=self.collection_name,
                    scroll_filter=qdrant_client.http.models.Filter(
                        must=[
                            qdrant_client.http.models.FieldCondition(
                                key="metadata.memory_id",
                                match=qdrant_client.http.models.MatchValue(
                                    value=memory_id
                                ),
                            ),
                        ],
                    ),
                    limit=100,
                    with_payload=True,
                    with_vectors=False,
                )

        try:
            records, _ = await asyncio.get_running_loop().run_in_executor(
                None,
                run_in_context,
                opentelemetry.context.get_current(),
            )

            if not records:
                return None

            sorted_records = sorted(
                records,
                key=lambda r: int(r.payload["id"].split("_")[-1])
                if "_" in r.payload["id"]
                else 0,
            )

            combined_text = " ".join(
                record.payload["text"] for record in sorted_records
            )
            return combined_text

        except qdrant_client.http.exceptions.UnexpectedResponse as e:
            match e.status_code:
                case 409 | 429 | 502 | 503 | 504:
                    raise memory_bank.exceptions.RetryableError(e) from e
            raise e
        except qdrant_client.http.exceptions.ResponseHandlingException as e:
            match type(e.source):
                case (
                    httpx.ConnectTimeout
                    | httpx.ReadTimeout
                    | httpx.WriteTimeout
                    | httpx.PoolTimeout
                    | httpx.ReadError
                    | httpx.WriteError
                    | httpx.ConnectError
                    | httpx.CloseError
                ):
                    raise memory_bank.exceptions.RetryableError(e) from e
            raise e

    async def _delete_existing_memory(self, memory_id: str) -> None:
        def run_in_context(ctx: opentelemetry.context.Context):
            opentelemetry.context.attach(ctx)
            with memory_bank.telemetry.tracer.start_as_current_span(
                "qdrant.delete_existing_memory"
            ):
                return self.client.delete(
                    collection_name=self.collection_name,
                    points_selector=qdrant_client.http.models.Filter(
                        must=[
                            qdrant_client.http.models.FieldCondition(
                                key="metadata.memory_id",
                                match=qdrant_client.http.models.MatchValue(
                                    value=memory_id
                                ),
                            ),
                        ],
                    ),
                )

        try:
            await asyncio.get_running_loop().run_in_executor(
                None,
                run_in_context,
                opentelemetry.context.get_current(),
            )
        except qdrant_client.http.exceptions.UnexpectedResponse as e:
            match e.status_code:
                case 409 | 429 | 502 | 503 | 504:
                    raise memory_bank.exceptions.RetryableError(e) from e
            raise e
        except qdrant_client.http.exceptions.ResponseHandlingException as e:
            match type(e.source):
                case (
                    httpx.ConnectTimeout
                    | httpx.ReadTimeout
                    | httpx.WriteTimeout
                    | httpx.PoolTimeout
                    | httpx.ReadError
                    | httpx.WriteError
                    | httpx.ConnectError
                    | httpx.CloseError
                ):
                    raise memory_bank.exceptions.RetryableError(e) from e
            raise e

    async def _upsert(
        self,
        memory_chunks_by_category: collections.abc.Mapping[
            str,
            collections.abc.Sequence[memory_bank.datastore.MemoryChunkWithEmbedding],
        ],
    ) -> None:
        delete_tasks = []
        for category, chunks in memory_chunks_by_category.items():
            if chunks:
                memory_id = str(uuid.uuid5(self.MEMORY_NAMESPACE, category))
                delete_tasks.append(self._delete_existing_memory(memory_id))

        if delete_tasks:
            await asyncio.gather(*delete_tasks)

        points = [
            qdrant_client.http.models.PointStruct(
                id=uuid.uuid5(self.UUID_NAMESPACE, chunk.id).hex,
                vector={
                    "openai": chunk.embedding,
                    "bm25": _convert_ndarray_tolist(
                        list(bm25.embed([chunk.text]))[0].as_object()
                    ),
                    "bm42": _convert_ndarray_tolist(
                        list(bm42.embed([chunk.text]))[0].as_object()
                    ),
                },
                payload={
                    "id": chunk.id,
                    "text": chunk.text,
                    "metadata": chunk.metadata.model_dump(),
                },
            )
            for _, chunks in memory_chunks_by_category.items()
            for chunk in chunks
        ]

        def run_in_context(ctx: opentelemetry.context.Context):
            opentelemetry.context.attach(ctx)
            with memory_bank.telemetry.tracer.start_as_current_span("qdrant.upsert"):
                return self.client.upsert(
                    collection_name=self.collection_name,
                    points=points,
                    wait=True,
                )

        try:
            await asyncio.get_running_loop().run_in_executor(
                None,
                run_in_context,
                opentelemetry.context.get_current(),
            )
        except qdrant_client.http.exceptions.UnexpectedResponse as e:
            match e.status_code:
                case 409 | 429 | 502 | 503 | 504:
                    raise memory_bank.exceptions.RetryableError(e) from e
            raise e
        except qdrant_client.http.exceptions.ResponseHandlingException as e:
            match type(e.source):
                case (
                    httpx.ConnectTimeout
                    | httpx.ReadTimeout
                    | httpx.WriteTimeout
                    | httpx.PoolTimeout
                    | httpx.ReadError
                    | httpx.WriteError
                    | httpx.ConnectError
                    | httpx.CloseError
                ):
                    raise memory_bank.exceptions.RetryableError(e) from e
            raise e

    async def _query(
        self,
        queries: collections.abc.Sequence[memory_bank.datastore.QueryWithEmbedding],
    ) -> collections.abc.Sequence[memory_bank.model.QueryResult]:
        requests = [
            qdrant_client.http.models.QueryRequest(
                prefetch=[
                    qdrant_client.http.models.Prefetch(
                        query=query.embedding,
                        using="openai",
                        limit=10,
                    ),
                    qdrant_client.http.models.Prefetch(
                        query=_convert_ndarray_tolist(
                            list(bm42.embed([query.query]))[0].as_object()
                        ),
                        using="bm42",
                        limit=10,
                    ),
                ],
                query=qdrant_client.http.models.FusionQuery(
                    fusion=qdrant_client.http.models.Fusion.RRF
                ),
                filter=_convert_metadata_filter_to_qdrant_filter(query.filter)
                if query.filter
                else None,
                limit=query.top_k,
                with_payload=True,
                with_vector=False,
            )
            for query in queries
        ]

        def run_in_context(ctx: opentelemetry.context.Context):
            opentelemetry.context.attach(ctx)
            with memory_bank.telemetry.tracer.start_as_current_span(
                "qdrant.query_batch_points"
            ):
                return self.client.query_batch_points(
                    collection_name=self.collection_name,
                    requests=requests,
                )

        try:
            results = await asyncio.get_running_loop().run_in_executor(
                None,
                run_in_context,
                opentelemetry.context.get_current(),
            )
        except qdrant_client.http.exceptions.UnexpectedResponse as e:
            match e.status_code:
                case 409 | 429 | 502 | 503 | 504:
                    raise memory_bank.exceptions.RetryableError(e) from e
            raise e
        except qdrant_client.http.exceptions.ResponseHandlingException as e:
            match type(e.source):
                case (
                    httpx.ConnectTimeout
                    | httpx.ReadTimeout
                    | httpx.WriteTimeout
                    | httpx.PoolTimeout
                    | httpx.ReadError
                    | httpx.WriteError
                    | httpx.ConnectError
                    | httpx.CloseError
                ):
                    raise memory_bank.exceptions.RetryableError(e) from e
            raise e

        return [
            memory_bank.model.QueryResult(
                query=query.query,
                results=[
                    memory_bank.model.MemoryChunkWithScore(
                        id=scored_point.payload.get("id"),
                        score=scored_point.score,
                        text=scored_point.payload.get("text"),
                        metadata=scored_point.payload.get("metadata"),
                    )
                    for scored_point in result.points
                ],
            )
            for query, result in zip(queries, results)
        ]

    async def _delete(
        self,
        metadata_filter: memory_bank.model.MemoryMetadataFilter,
    ) -> bool:
        def run_in_context(ctx: opentelemetry.context.Context):
            opentelemetry.context.attach(ctx)
            with memory_bank.telemetry.tracer.start_as_current_span("qdrant.delete"):
                return self.client.delete(
                    collection_name=self.collection_name,
                    points_selector=_convert_metadata_filter_to_qdrant_filter(
                        metadata_filter
                    ),
                )

        try:
            response = await asyncio.get_running_loop().run_in_executor(
                None,
                run_in_context,
                opentelemetry.context.get_current(),
            )
        except qdrant_client.http.exceptions.UnexpectedResponse as e:
            match e.status_code:
                case 409 | 429 | 502 | 503 | 504:
                    raise memory_bank.exceptions.RetryableError(e) from e
            raise e
        except qdrant_client.http.exceptions.ResponseHandlingException as e:
            match type(e.source):
                case (
                    httpx.ConnectTimeout
                    | httpx.ReadTimeout
                    | httpx.WriteTimeout
                    | httpx.PoolTimeout
                    | httpx.ReadError
                    | httpx.WriteError
                    | httpx.ConnectError
                    | httpx.CloseError
                ):
                    raise memory_bank.exceptions.RetryableError(e) from e
            raise e

        return "completed" == response.status

    async def _get_memory_by_category(
        self, category: str
    ) -> memory_bank.model.MemoryChunkWithScore | None:
        def run_in_context(ctx: opentelemetry.context.Context):
            opentelemetry.context.attach(ctx)
            with memory_bank.telemetry.tracer.start_as_current_span(
                "qdrant.get_memory_by_category"
            ):
                memory_id = str(
                    uuid.uuid5(
                        memory_bank.datastore.DataStore.MEMORY_NAMESPACE, category
                    )
                )
                scroll_filter = qdrant_client.http.models.Filter(
                    must=[
                        qdrant_client.http.models.FieldCondition(
                            key="metadata.memory_id",
                            match=qdrant_client.http.models.MatchValue(value=memory_id),
                        ),
                    ],
                )

                all_records = []
                offset = None

                while True:
                    records, next_offset = self.client.scroll(
                        collection_name=self.collection_name,
                        scroll_filter=scroll_filter,
                        limit=100,
                        offset=offset,
                        with_payload=True,
                        with_vectors=False,
                    )

                    all_records.extend(records)

                    if next_offset is None:
                        break
                    offset = next_offset

                return all_records

        try:
            records = await asyncio.get_running_loop().run_in_executor(
                None,
                run_in_context,
                opentelemetry.context.get_current(),
            )

            if not records:
                return None

            sorted_records = sorted(
                records,
                key=lambda r: int(r.payload["id"].split("_")[-1])
                if "_" in r.payload["id"]
                else 0,
            )

            combined_text = " ".join(
                record.payload["text"] for record in sorted_records
            )

            first_record = sorted_records[0]
            memory_id = first_record.payload["metadata"]["memory_id"]

            return memory_bank.model.MemoryChunkWithScore(
                id=memory_id,
                score=1.0,
                text=combined_text,
                metadata=first_record.payload["metadata"],
            )

        except qdrant_client.http.exceptions.UnexpectedResponse as e:
            match e.status_code:
                case 409 | 429 | 502 | 503 | 504:
                    raise memory_bank.exceptions.RetryableError(e) from e
            raise e
        except qdrant_client.http.exceptions.ResponseHandlingException as e:
            match type(e.source):
                case (
                    httpx.ConnectTimeout
                    | httpx.ReadTimeout
                    | httpx.WriteTimeout
                    | httpx.PoolTimeout
                    | httpx.ReadError
                    | httpx.WriteError
                    | httpx.ConnectError
                    | httpx.CloseError
                ):
                    raise memory_bank.exceptions.RetryableError(e) from e
            raise e
