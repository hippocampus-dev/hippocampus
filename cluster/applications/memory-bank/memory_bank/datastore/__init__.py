import abc
import asyncio
import collections.abc
import os
import uuid

import httpx
import memory_bank.categorizer
import memory_bank.exceptions
import memory_bank.llm.openai
import memory_bank.merger
import memory_bank.model
import memory_bank.settings
import openai
import tiktoken


class QueryWithEmbedding(memory_bank.model.Query):
    embedding: collections.abc.Sequence[float]


class MemoryChunkWithEmbedding(memory_bank.model.MemoryChunk):
    embedding: collections.abc.Sequence[float]


class DataStore(abc.ABC):
    MEMORY_NAMESPACE = uuid.UUID("a7c4b2d1-9e8f-4a3b-8c5d-6e7f8a9b0c1d")

    default_chunk_size: int
    embedding_batch_size: int
    embedding_model: memory_bank.settings.EmbeddingModel
    encoder: tiktoken.Encoding
    categorizer: memory_bank.categorizer.Categorizer
    merger: memory_bank.merger.Merger

    def __init__(
        self,
        default_chunk_size: int,
        embedding_batch_size: int,
        embedding_model: memory_bank.settings.EmbeddingModel,
        encoder: tiktoken.Encoding,
        categorizer: memory_bank.categorizer.Categorizer,
        merger: memory_bank.merger.Merger,
    ):
        self.default_chunk_size = default_chunk_size
        self.embedding_batch_size = embedding_batch_size
        self.embedding_model = embedding_model
        self.encoder = encoder
        self.categorizer = categorizer
        self.merger = merger

    async def init(self):
        pass

    async def upsert(
        self,
        documents: collections.abc.Sequence[memory_bank.model.Document],
        categories: collections.abc.Sequence[str],
    ) -> collections.abc.Sequence[str]:
        document_texts = [document.text for document in documents]
        document_category_segments = await self.categorizer.categorize_documents(
            document_texts, categories
        )

        category_texts: collections.abc.MutableMapping[
            str, collections.abc.MutableSequence[str]
        ] = {}

        for document_index, category_content_map in document_category_segments.items():
            for category, content in category_content_map.items():
                if category not in category_texts:
                    category_texts[category] = []
                category_texts[category].append(content)

        async def process_category(
            category: str,
        ) -> tuple[str, collections.abc.Sequence[memory_bank.model.MemoryChunk]]:
            memory_id = str(uuid.uuid5(self.MEMORY_NAMESPACE, category))

            texts_for_category = category_texts.get(category, [])
            if not texts_for_category:
                return memory_id, []

            category_text = " ".join(texts_for_category)

            existing_memory = await self._get_existing_memory(memory_id)
            if existing_memory:
                category_text = await self.merger.merge_memories(
                    existing_memory,
                    category_text,
                    category,
                )

            text_chunks = self._get_text_chunks(
                category_text,
                min(
                    documents[0].chunk_size
                    if documents and documents[0].chunk_size
                    else self.default_chunk_size,
                    self.embedding_model.max_tokens,
                ),
            )

            memory_chunks = []
            for i, text_chunk in enumerate(text_chunks):
                chunk_id = f"{memory_id}_{i}"
                memory_chunk = memory_bank.model.MemoryChunk(
                    id=chunk_id,
                    text=text_chunk.replace("\n", " ").strip(),
                    metadata=memory_bank.model.MemoryMetadata(
                        memory_id=memory_id,
                        category=category,
                        source=None,
                        source_id=None,
                    ),
                )
                memory_chunks.append(memory_chunk)

            return memory_id, memory_chunks

        category_results = await asyncio.gather(
            *[process_category(category) for category in categories]
        )

        all_memory_chunks: collections.abc.MutableSequence[
            memory_bank.model.MemoryChunk
        ] = []
        memory_ids: collections.abc.MutableSequence[str] = []

        for memory_id, chunks in category_results:
            memory_ids.append(memory_id)
            all_memory_chunks.extend(chunks)

        embeddings: collections.abc.MutableSequence[
            collections.abc.Sequence[float]
        ] = []
        for i in range(0, len(all_memory_chunks), self.embedding_batch_size):
            texts = [
                chunk.text
                for chunk in all_memory_chunks[i : (i + self.embedding_batch_size)]
            ]

            try:
                response = await memory_bank.llm.openai.AsyncOpenAI(
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
                    input=texts,
                    model=self.embedding_model,
                    dimensions=memory_bank.settings.OPENAI_VECTOR_SIZE
                    if self.embedding_model
                    in (
                        memory_bank.settings.EmbeddingModel.ADA_V3_SMALL,
                        memory_bank.settings.EmbeddingModel.ADA_V3_LARGE,
                    )
                    else openai._types.NOT_GIVEN,
                )
            except openai.APIConnectionError as e:
                raise memory_bank.exceptions.RetryableError(e) from e
            except openai.APIStatusError as e:
                match e.status_code:
                    case 404:
                        if e.message == "Engine not found":
                            raise memory_bank.exceptions.RetryableError(e) from e
                    case 409 | 429 | 502 | 503 | 504:
                        raise memory_bank.exceptions.RetryableError(e) from e
                raise e

            embeddings.extend([result.embedding for result in response.data])

        memory_chunks_by_category: collections.abc.MutableMapping[
            str, collections.abc.MutableSequence[MemoryChunkWithEmbedding]
        ] = {}
        for i, chunk in enumerate(all_memory_chunks):
            category = chunk.metadata.category
            if category not in memory_chunks_by_category:
                memory_chunks_by_category[category] = []
            memory_chunks_by_category[category].append(
                MemoryChunkWithEmbedding(embedding=embeddings[i], **chunk.model_dump()),
            )

        await self._upsert(memory_chunks_by_category)
        return memory_ids

    @abc.abstractmethod
    async def _get_existing_memory(self, memory_id: str) -> str | None:
        raise NotImplementedError

    @abc.abstractmethod
    async def _upsert(
        self,
        memory_chunks_by_category: collections.abc.Mapping[
            str, collections.abc.Sequence[MemoryChunkWithEmbedding]
        ],
    ) -> None:
        raise NotImplementedError

    async def query(
        self,
        queries: collections.abc.Sequence[memory_bank.model.Query],
    ) -> collections.abc.Sequence[memory_bank.model.QueryResult]:
        empty_queries = []
        other_queries = []

        for query in queries:
            if query.query == "":
                if not (query.filter and query.filter.category):
                    raise ValueError(
                        "Empty query string requires a category filter to retrieve all memories for that category"
                    )
                empty_queries.append(query)
            else:
                other_queries.append(query)

        results = []

        for query in empty_queries:
            memory = await self._get_memory_by_category(query.filter.category)
            results.append(
                memory_bank.model.QueryResult(
                    query=query.query,
                    results=[memory] if memory else [],
                )
            )

        if other_queries:
            texts = [query.query for query in other_queries]

            try:
                response = await memory_bank.llm.openai.AsyncOpenAI(
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
                    input=texts,
                    model=self.embedding_model,
                    dimensions=memory_bank.settings.OPENAI_VECTOR_SIZE
                    if self.embedding_model
                    in (
                        memory_bank.settings.EmbeddingModel.ADA_V3_SMALL,
                        memory_bank.settings.EmbeddingModel.ADA_V3_LARGE,
                    )
                    else openai._types.NOT_GIVEN,
                )
            except openai.APIConnectionError as e:
                raise memory_bank.exceptions.RetryableError(e) from e
            except openai.APIStatusError as e:
                match e.status_code:
                    case 404:
                        if e.message == "Engine not found":
                            raise memory_bank.exceptions.RetryableError(e) from e
                    case 409 | 429 | 502 | 503 | 504:
                        raise memory_bank.exceptions.RetryableError(e) from e
                raise e

            queries_with_embeddings = [
                QueryWithEmbedding(
                    embedding=embedding,
                    **query.model_dump(),
                )
                for query, embedding in zip(
                    other_queries, [result.embedding for result in response.data]
                )
            ]
            normal_results = await self._query(queries_with_embeddings)
            results.extend(normal_results)

        return results

    @abc.abstractmethod
    async def _query(
        self, queries: collections.abc.Sequence[QueryWithEmbedding]
    ) -> collections.abc.Sequence[memory_bank.model.QueryResult]:
        raise NotImplementedError

    @abc.abstractmethod
    async def _get_memory_by_category(
        self, category: str
    ) -> memory_bank.model.MemoryChunkWithScore | None:
        raise NotImplementedError

    async def delete(
        self, metadata_filter: memory_bank.model.MemoryMetadataFilter
    ) -> bool:
        return await self._delete(metadata_filter)

    @abc.abstractmethod
    async def _delete(
        self, metadata_filter: memory_bank.model.MemoryMetadataFilter
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
