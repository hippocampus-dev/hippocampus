import collections.abc

import pydantic


class DocumentMetadata(pydantic.BaseModel):
    source: str | None = None
    source_id: str | None = None


class DocumentChunkMetadata(DocumentMetadata):
    document_id: str | None = None


class DocumentMetadataFilter(DocumentChunkMetadata):
    pass


# Upsert

class Document(pydantic.BaseModel):
    id: str
    text: str
    chunk_size: int | None = None
    metadata: DocumentMetadata


class UpsertRequest(pydantic.BaseModel):
    documents: collections.abc.Sequence[Document]


class UpsertResponse(pydantic.BaseModel):
    ids: collections.abc.Sequence[str]


# Query

class DocumentChunk(pydantic.BaseModel):
    id: str
    text: str
    metadata: DocumentChunkMetadata


class DocumentChunkWithScore(DocumentChunk):
    score: float


class Query(pydantic.BaseModel):
    query: str
    filter: DocumentMetadataFilter | None = None
    top_k: int | None = 3


class QueryRequest(pydantic.BaseModel):
    queries: collections.abc.Sequence[Query]


class QueryResult(pydantic.BaseModel):
    query: str
    results: collections.abc.Sequence[DocumentChunkWithScore]


class QueryResponse(pydantic.BaseModel):
    results: collections.abc.Sequence[QueryResult]


# Delete

class DeleteRequest(pydantic.BaseModel):
    filter: DocumentMetadataFilter


class DeleteResponse(pydantic.BaseModel):
    success: bool
