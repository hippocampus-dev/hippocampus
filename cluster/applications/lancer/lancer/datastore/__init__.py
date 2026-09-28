import abc
import collections.abc
import os

import httpx
import openai
import tiktoken

import lancer.exceptions
import lancer.model
import lancer.settings


class KnowledgeChunkWithEmbedding(lancer.model.KnowledgeChunk):
    embedding: collections.abc.Sequence[float]


class DataStore(abc.ABC):
    default_chunk_size: int
    embedding_batch_size: int
    embedding_model: lancer.settings.EmbeddingModel
    encoder: tiktoken.Encoding
    entity_similarity_threshold: float

    def __init__(
        self,
        default_chunk_size: int,
        embedding_batch_size: int,
        embedding_model: lancer.settings.EmbeddingModel,
        encoder: tiktoken.Encoding,
        entity_similarity_threshold: float,
    ):
        self.default_chunk_size = default_chunk_size
        self.embedding_batch_size = embedding_batch_size
        self.embedding_model = embedding_model
        self.encoder = encoder
        self.entity_similarity_threshold = entity_similarity_threshold

    async def init(self):
        pass

    async def _generate_embeddings(
        self,
        texts: collections.abc.Sequence[str],
    ) -> collections.abc.Sequence[collections.abc.Sequence[float]]:
        embeddings: collections.abc.MutableSequence[
            collections.abc.Sequence[float]
        ] = []

        for i in range(0, len(texts), self.embedding_batch_size):
            batch = texts[i : (i + self.embedding_batch_size)]

            try:
                response = await AsyncOpenAI(
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
                ).embeddings.create(
                    input=batch,
                    model=self.embedding_model,
                    dimensions=lancer.settings.OPENAI_VECTOR_SIZE
                    if self.embedding_model
                    in (
                        lancer.settings.EmbeddingModel.ADA_V3_SMALL,
                        lancer.settings.EmbeddingModel.ADA_V3_LARGE,
                    )
                    else openai._types.NOT_GIVEN,
                )
            except openai.APIConnectionError as e:
                raise lancer.exceptions.RetryableError(e) from e
            except openai.APIStatusError as e:
                match e.status_code:
                    case 404:
                        if e.message == "Engine not found":
                            raise lancer.exceptions.RetryableError(e) from e
                    case 409 | 429 | 502 | 503 | 504:
                        raise lancer.exceptions.RetryableError(e) from e
                raise e

            embeddings.extend([result.embedding for result in response.data])

        return embeddings

    async def upsert(
        self,
        request: lancer.model.UpsertRequest,
    ) -> lancer.model.UpsertResponse:
        chunk_size = min(
            request.chunk_size or self.default_chunk_size,
            self.embedding_model.max_tokens,
        )
        text_chunks = self._get_text_chunks(request.text, chunk_size)

        metadata = request.metadata or lancer.model.KnowledgeMetadata()

        chunks = [
            lancer.model.KnowledgeChunk(
                id=f"{request.document_id}_{i}",
                text=text_chunk.replace("\n", " ").strip(),
                metadata=lancer.model.KnowledgeChunkMetadata(
                    document_id=request.document_id,
                    **dict(metadata),
                ),
            )
            for i, text_chunk in enumerate(text_chunks)
        ]

        chunk_texts = [chunk.text for chunk in chunks]
        chunk_embeddings = await self._generate_embeddings(chunk_texts)

        chunks_with_embeddings = [
            KnowledgeChunkWithEmbedding(embedding=embedding, **dict(chunk))
            for chunk, embedding in zip(chunks, chunk_embeddings)
        ]

        chunk_ids = await self._upsert_knowledge(chunks_with_embeddings)

        entity_ids: collections.abc.MutableSequence[str] = []
        if request.entities:
            entity_texts = [
                f"{entity.name}: {entity.description or ''}"
                for entity in request.entities
            ]
            entity_embeddings = await self._generate_embeddings(entity_texts)

            entity_ids = await self._upsert_entities(
                request.entities, entity_embeddings, chunk_ids
            )

        relation_ids: collections.abc.MutableSequence[str] = []
        if request.relations:
            relation_ids = await self._upsert_relations(request.relations)

        return lancer.model.UpsertResponse(
            chunk_ids=chunk_ids,
            entity_ids=entity_ids,
            relation_ids=relation_ids,
        )

    @abc.abstractmethod
    async def _upsert_knowledge(
        self,
        chunks: collections.abc.Sequence[KnowledgeChunkWithEmbedding],
    ) -> collections.abc.Sequence[str]:
        raise NotImplementedError

    @abc.abstractmethod
    async def _upsert_entities(
        self,
        entities: collections.abc.Sequence[lancer.model.Entity],
        embeddings: collections.abc.Sequence[collections.abc.Sequence[float]],
        chunk_ids: collections.abc.Sequence[str],
    ) -> collections.abc.Sequence[str]:
        raise NotImplementedError

    @abc.abstractmethod
    async def _upsert_relations(
        self,
        relations: collections.abc.Sequence[lancer.model.Relation],
    ) -> collections.abc.Sequence[str]:
        raise NotImplementedError

    async def query(
        self,
        request: lancer.model.QueryRequest,
    ) -> lancer.model.QueryResponse:
        query_embeddings = await self._generate_embeddings([request.query])
        return await self._query(
            request.query,
            query_embeddings[0],
            request.filter,
            request.top_k or 5,
            request.graph_depth or 2,
        )

    @abc.abstractmethod
    async def _query(
        self,
        query: str,
        embedding: collections.abc.Sequence[float],
        metadata_filter: lancer.model.KnowledgeMetadataFilter | None,
        top_k: int,
        graph_depth: int,
    ) -> lancer.model.QueryResponse:
        raise NotImplementedError

    async def delete(
        self, metadata_filter: lancer.model.KnowledgeMetadataFilter
    ) -> bool:
        return await self._delete(metadata_filter)

    @abc.abstractmethod
    async def _delete(
        self, metadata_filter: lancer.model.KnowledgeMetadataFilter
    ) -> bool:
        raise NotImplementedError

    def _get_text_chunks(
        self, text: str, chunk_size: int
    ) -> collections.abc.Sequence[str]:
        tokens = self.encoder.encode(text, disallowed_special=())
        if len(tokens) <= chunk_size:
            return [text]

        chunks = []

        while tokens:
            chunk_text = self._punctuate(self.encoder.decode(tokens[:chunk_size]))

            tokens = tokens[
                len(self.encoder.encode(chunk_text, disallowed_special=())) :
            ]

            if not chunk_text.strip():
                continue

            chunks.append(chunk_text)

        return chunks

    def _punctuate(self, text: str) -> str:
        last_punctuation = max(
            text.rfind("."),
            text.rfind("．"),
            text.rfind("。"),
            text.rfind("?"),
            text.rfind("？"),
            text.rfind("!"),
            text.rfind("！"),
            text.rfind("\n"),
        )

        if last_punctuation != -1:
            return text[: (last_punctuation + 1)]

        return text


def AsyncOpenAI(*args, **kwargs) -> openai.AsyncOpenAI:
    if os.getenv("OPENAI_API_TYPE") == "azure":
        return openai.AsyncAzureOpenAI(
            *args,
            **kwargs,
        )
    return openai.AsyncOpenAI(
        *args,
        **kwargs,
    )
