import collections.abc
import typing

import pydantic


class DocumentMetadata(pydantic.BaseModel):
    source: str | None = None
    source_id: str | None = None
    created_at: float | None = None
    updated_at: float | None = None


class DocumentChunkMetadata(DocumentMetadata):
    document_id: str | None = None


class Range(pydantic.BaseModel):
    gte: float | None = None
    gt: float | None = None
    lte: float | None = None
    lt: float | None = None

    @pydantic.model_validator(mode="after")
    def validate_at_least_one(self) -> typing.Self:
        if all(v is None for v in [self.gte, self.gt, self.lte, self.lt]):
            raise ValueError("at least one range bound must be specified")
        return self


class DocumentMetadataFilter(pydantic.BaseModel):
    source: str | None = None
    source_any: collections.abc.Sequence[str] | None = None
    source_not: collections.abc.Sequence[str] | None = None
    source_id: str | None = None
    document_id: str | None = None
    created_at: Range | None = None
    updated_at: Range | None = None


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
