import abc
import asyncio
import collections.abc
import uuid

import fastembed
import httpx
import numpy
import opentelemetry.context
import qdrant_client.http.exceptions
import qdrant_client.http.models
import tiktoken

import embedding_retrieval.datastore
import embedding_retrieval.exceptions
import embedding_retrieval.model
import embedding_retrieval.settings
import embedding_retrieval.telemetry

# https://github.com/qdrant/fastembed/blob/331207976e771e43fd92531c2236deb0e1a51ad0/fastembed/sparse/sparse_text_embedding.py#L14
bm25 = fastembed.SparseTextEmbedding("Qdrant/bm25")
bm42 = fastembed.SparseTextEmbedding("Qdrant/bm42-all-minilm-l6-v2-attentions")


def _convert_metadata_filter_to_qdrant_filter(
    metadata_filter: embedding_retrieval.model.DocumentMetadataFilter,
) -> qdrant_client.http.models.Filter:
    must_conditions = []

    for key, value in metadata_filter.dict().items():
        if value is not None:
            must_conditions.append(
                qdrant_client.http.models.FieldCondition(
                    key=f"metadata.{key}", match=qdrant_client.http.models.MatchValue(value=value),
                ),
            )

    return qdrant_client.http.models.Filter(must=must_conditions, should=[])


def _convert_ndarray_tolist(
    m: collections.abc.Mapping[str, numpy.ndarray],
) -> collections.abc.Mapping[str, collections.abc.Sequence]:
    return {k: v.tolist() for k, v in m.items()}


class QdrantDataStore(embedding_retrieval.datastore.DataStore, abc.ABC):
    UUID_NAMESPACE = uuid.UUID("3896d314-1e95-4a3a-b45a-945f9f0b541d")

    def __init__(
        self,
        default_chunk_size: int,
        embedding_batch_size: int,
        embedding_model: embedding_retrieval.settings.EmbeddingModel,
        encoder: tiktoken.Encoding,
        host: str,
        port: int,
        timeout: int,
        collection_name: str,
        replication_factor: int,
        shard_number: int,
    ):
        super().__init__(default_chunk_size, embedding_batch_size, embedding_model, encoder)

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
                                size=embedding_retrieval.settings.OPENAI_VECTOR_SIZE,
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
                        field_name="metadata.document_id",
                        field_type=qdrant_client.http.models.PayloadSchemaType.KEYWORD,
                    )
                case 409 | 429 | 502 | 503 | 504:
                    raise embedding_retrieval.exceptions.RetryableError(e) from e

    async def _upsert(
        self,
        document_chunks: collections.abc.Mapping[
            str, collections.abc.Sequence[embedding_retrieval.datastore.DocumentChunkWithEmbedding]
        ],
    ) -> collections.abc.Sequence[str]:
        points = [
            qdrant_client.http.models.PointStruct(
                id=uuid.uuid5(self.UUID_NAMESPACE, chunk.id).hex,
                vector={
                    "openai": chunk.embedding,
                    "bm25": _convert_ndarray_tolist(list(bm25.embed([chunk.text]))[0].as_object()),
                    "bm42": _convert_ndarray_tolist(list(bm42.embed([chunk.text]))[0].as_object()),
                },
                payload={
                    "id": chunk.id,
                    "text": chunk.text,
                    "metadata": chunk.metadata.dict(),
                },
            )
            for _, chunks in document_chunks.items()
            for chunk in chunks
        ]

        def run_in_context(ctx: opentelemetry.context.Context):
            opentelemetry.context.attach(ctx)
            with embedding_retrieval.telemetry.tracer.start_as_current_span("qdrant.upsert"):
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
                    raise embedding_retrieval.exceptions.RetryableError(e) from e
            raise e
        except qdrant_client.http.exceptions.ResponseHandlingException as e:
            match type(e.source):
                case httpx.ConnectTimeout | httpx.ReadTimeout | httpx.WriteTimeout | httpx.PoolTimeout | httpx.ReadError | httpx.WriteError | httpx.ConnectError | httpx.CloseError:
                    raise embedding_retrieval.exceptions.RetryableError(e) from e
            raise e

        return list(document_chunks.keys())

    async def _query(
        self,
        queries: collections.abc.Sequence[embedding_retrieval.datastore.QueryWithEmbedding],
    ) -> collections.abc.Sequence[embedding_retrieval.model.QueryResult]:
        requests = [
            qdrant_client.http.models.QueryRequest(
                prefetch=[
                    qdrant_client.http.models.Prefetch(
                        query=query.embedding, using="openai", limit=10,
                    ),
                    qdrant_client.http.models.Prefetch(
                        query=_convert_ndarray_tolist(list(bm42.embed([query.query]))[0].as_object()), using="bm42",
                        limit=10,
                    ),
                ],
                query=qdrant_client.http.models.FusionQuery(fusion=qdrant_client.http.models.Fusion.RRF),
                filter=_convert_metadata_filter_to_qdrant_filter(query.filter) if query.filter else None,
                limit=query.top_k,
                with_payload=True,
                with_vector=False,
            ) for query in queries
        ]

        def run_in_context(ctx: opentelemetry.context.Context):
            opentelemetry.context.attach(ctx)
            with embedding_retrieval.telemetry.tracer.start_as_current_span("qdrant.query_batch_points"):
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
                    raise embedding_retrieval.exceptions.RetryableError(e) from e
            raise e
        except qdrant_client.http.exceptions.ResponseHandlingException as e:
            match type(e.source):
                case httpx.ConnectTimeout | httpx.ReadTimeout | httpx.WriteTimeout | httpx.PoolTimeout | httpx.ReadError | httpx.WriteError | httpx.ConnectError | httpx.CloseError:
                    raise embedding_retrieval.exceptions.RetryableError(e) from e
            raise e

        return [
            embedding_retrieval.model.QueryResult(
                query=query.query,
                results=[
                    embedding_retrieval.model.DocumentChunkWithScore(
                        id=scored_point.payload.get("id"),  # scored_point.id has been converted to uuid format
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
        metadata_filter: embedding_retrieval.model.DocumentMetadataFilter,
    ) -> bool:
        def run_in_context(ctx: opentelemetry.context.Context):
            opentelemetry.context.attach(ctx)
            with embedding_retrieval.telemetry.tracer.start_as_current_span("qdrant.delete"):
                return self.client.delete(
                    collection_name=self.collection_name,
                    points_selector=_convert_metadata_filter_to_qdrant_filter(metadata_filter),
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
                    raise embedding_retrieval.exceptions.RetryableError(e) from e
            raise e
        except qdrant_client.http.exceptions.ResponseHandlingException as e:
            match type(e.source):
                case httpx.ConnectTimeout | httpx.ReadTimeout | httpx.WriteTimeout | httpx.PoolTimeout | httpx.ReadError | httpx.WriteError | httpx.ConnectError | httpx.CloseError:
                    raise embedding_retrieval.exceptions.RetryableError(e) from e
            raise e

        return "completed" == response.status
