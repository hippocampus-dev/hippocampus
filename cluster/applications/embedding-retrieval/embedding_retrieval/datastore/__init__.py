import abc
import collections.abc
import os

import httpx
import openai
import tiktoken

import embedding_retrieval.exceptions
import embedding_retrieval.model
import embedding_retrieval.settings


class QueryWithEmbedding(embedding_retrieval.model.Query):
    embedding: collections.abc.Sequence[float]


class DocumentChunkWithEmbedding(embedding_retrieval.model.DocumentChunk):
    embedding: collections.abc.Sequence[float]


class DataStore(abc.ABC):
    default_chunk_size: int
    embedding_batch_size: int
    embedding_model: embedding_retrieval.settings.EmbeddingModel
    encoder: tiktoken.Encoding

    def __init__(
        self,
        default_chunk_size: int,
        embedding_batch_size: int,
        embedding_model: embedding_retrieval.settings.EmbeddingModel,
        encoder: tiktoken.Encoding,
    ):
        self.default_chunk_size = default_chunk_size
        self.embedding_batch_size = embedding_batch_size
        self.embedding_model = embedding_model
        self.encoder = encoder

    async def init(self):
        pass

    async def upsert(
        self,
        documents: collections.abc.Sequence[embedding_retrieval.model.Document],
    ) -> collections.abc.Sequence[str]:
        all_chunks: collections.abc.MutableSequence[
            embedding_retrieval.model.DocumentChunk
        ] = []

        for document in documents:
            text_chunks = self._get_text_chunks(
                document.text,
                min(
                    document.chunk_size or self.default_chunk_size,
                    self.embedding_model.max_tokens,
                ),
            )

            chunks = [
                embedding_retrieval.model.DocumentChunk(
                    id=f"{document.id}_{i}",
                    text=text_chunk.replace("\n", " ").strip(),
                    metadata=embedding_retrieval.model.DocumentChunkMetadata(
                        document_id=document.id,
                        **dict(document.metadata),
                    ),
                )
                for i, text_chunk in enumerate(text_chunks)
            ]

            all_chunks.extend(chunks)

        embeddings: collections.abc.MutableSequence[
            collections.abc.Sequence[float]
        ] = []
        for i in range(0, len(all_chunks), self.embedding_batch_size):
            texts = [
                chunk.text for chunk in all_chunks[i : (i + self.embedding_batch_size)]
            ]

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
                    input=texts,
                    model=self.embedding_model,
                    dimensions=embedding_retrieval.settings.OPENAI_VECTOR_SIZE
                    if self.embedding_model
                    in (
                        embedding_retrieval.settings.EmbeddingModel.ADA_V3_SMALL,
                        embedding_retrieval.settings.EmbeddingModel.ADA_V3_LARGE,
                    )
                    else openai._types.NOT_GIVEN,
                )
            except openai.APIConnectionError as e:
                raise embedding_retrieval.exceptions.RetryableError(e) from e
            except openai.APIStatusError as e:
                match e.status_code:
                    case 404:
                        if e.message == "Engine not found":
                            raise embedding_retrieval.exceptions.RetryableError(
                                e
                            ) from e
                    case 409 | 429 | 502 | 503 | 504:
                        raise embedding_retrieval.exceptions.RetryableError(e) from e
                raise e

            embeddings.extend([result.embedding for result in response.data])

        document_chunks: collections.abc.MutableMapping[
            str, collections.abc.MutableSequence[DocumentChunkWithEmbedding]
        ] = {}
        for i, chunk in enumerate(all_chunks):
            if chunk.metadata.document_id not in document_chunks:
                document_chunks[chunk.metadata.document_id] = []
            document_chunks[chunk.metadata.document_id].append(
                DocumentChunkWithEmbedding(embedding=embeddings[i], **dict(chunk)),
            )

        return await self._upsert(document_chunks)

    @abc.abstractmethod
    async def _upsert(
        self,
        document_chunks: collections.abc.Mapping[
            str, collections.abc.Sequence[DocumentChunkWithEmbedding]
        ],
    ) -> collections.abc.Sequence[str]:
        raise NotImplementedError

    async def query(
        self,
        queries: collections.abc.Sequence[embedding_retrieval.model.Query],
    ) -> collections.abc.Sequence[embedding_retrieval.model.QueryResult]:
        texts = [query.query for query in queries]

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
                input=texts,
                model=self.embedding_model,
                dimensions=embedding_retrieval.settings.OPENAI_VECTOR_SIZE
                if self.embedding_model
                in (
                    embedding_retrieval.settings.EmbeddingModel.ADA_V3_SMALL,
                    embedding_retrieval.settings.EmbeddingModel.ADA_V3_LARGE,
                )
                else openai._types.NOT_GIVEN,
            )
        except openai.APIConnectionError as e:
            raise embedding_retrieval.exceptions.RetryableError(e) from e
        except openai.APIStatusError as e:
            match e.status_code:
                case 404:
                    if e.message == "Engine not found":
                        raise embedding_retrieval.exceptions.RetryableError(e) from e
                case 409 | 429 | 502 | 503 | 504:
                    raise embedding_retrieval.exceptions.RetryableError(e) from e
            raise e

        queries_with_embeddings = [
            QueryWithEmbedding(
                embedding=embedding,
                **dict(query),
            )
            for query, embedding in zip(
                queries, [result.embedding for result in response.data]
            )
        ]
        return await self._query(queries_with_embeddings)

    @abc.abstractmethod
    async def _query(
        self, queries: collections.abc.Sequence[QueryWithEmbedding]
    ) -> collections.abc.Sequence[embedding_retrieval.model.QueryResult]:
        raise NotImplementedError

    async def delete(
        self, metadata_filter: embedding_retrieval.model.DocumentMetadataFilter
    ) -> bool:
        return await self._delete(metadata_filter)

    @abc.abstractmethod
    async def _delete(
        self, metadata_filter: embedding_retrieval.model.DocumentMetadataFilter
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

            if chunk_text.strip() is None:
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
