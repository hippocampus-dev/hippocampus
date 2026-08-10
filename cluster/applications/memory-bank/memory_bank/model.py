import collections.abc

import pydantic


class MemoryMetadata(pydantic.BaseModel):
    memory_id: str
    category: str
    source: str | None = None
    source_id: str | None = None


class MemoryMetadataFilter(pydantic.BaseModel):
    memory_id: str | None = None
    category: str | None = None
    source: str | None = None
    source_id: str | None = None


# Upsert


class Document(pydantic.BaseModel):
    text: str
    chunk_size: int | None = None


class UpsertRequest(pydantic.BaseModel):
    documents: collections.abc.Sequence[Document]
    categories: collections.abc.Sequence[str]


class UpsertResponse(pydantic.BaseModel):
    ids: collections.abc.Sequence[str]


# Query


class Memory(pydantic.BaseModel):
    id: str
    text: str
    metadata: MemoryMetadata


class MemoryChunk(pydantic.BaseModel):
    id: str
    text: str
    metadata: MemoryMetadata


class MemoryChunkWithScore(MemoryChunk):
    score: float


class Query(pydantic.BaseModel):
    query: str
    filter: MemoryMetadataFilter | None = None
    top_k: int | None = 3


class QueryRequest(pydantic.BaseModel):
    queries: collections.abc.Sequence[Query]


class QueryResult(pydantic.BaseModel):
    query: str
    results: collections.abc.Sequence[MemoryChunkWithScore]


class QueryResponse(pydantic.BaseModel):
    results: collections.abc.Sequence[QueryResult]


# Delete


class DeleteRequest(pydantic.BaseModel):
    filter: MemoryMetadataFilter


class DeleteResponse(pydantic.BaseModel):
    success: bool
