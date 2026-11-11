import collections.abc

import pydantic


# Entity and Relation


class Entity(pydantic.BaseModel):
    name: str
    entity_type: str
    description: str | None = None
    metadata: dict[str, str] | None = None


class Relation(pydantic.BaseModel):
    source_entity: str
    target_entity: str
    relation_type: str
    context: str | None = None


# Knowledge Metadata


class KnowledgeMetadata(pydantic.BaseModel):
    source: str | None = None
    source_id: str | None = None
    created_at: float | None = None
    updated_at: float | None = None


class KnowledgeChunkMetadata(KnowledgeMetadata):
    document_id: str | None = None


class KnowledgeMetadataFilter(pydantic.BaseModel):
    source: str | None = None
    source_id: str | None = None
    document_id: str | None = None
    entity_name: str | None = None

    @pydantic.model_validator(mode="after")
    def validate_at_least_one(self) -> "KnowledgeMetadataFilter":
        if all(
            v is None
            for v in [self.source, self.source_id, self.document_id, self.entity_name]
        ):
            raise ValueError("at least one filter field must be specified")
        return self


# Upsert


class UpsertRequest(pydantic.BaseModel):
    document_id: str = pydantic.Field(min_length=1)
    text: str = pydantic.Field(min_length=1)
    chunk_size: int | None = pydantic.Field(default=None, gt=0)
    metadata: KnowledgeMetadata | None = None
    entities: collections.abc.Sequence[Entity] | None = None
    relations: collections.abc.Sequence[Relation] | None = None


class UpsertResponse(pydantic.BaseModel):
    chunk_ids: collections.abc.Sequence[str]
    entity_ids: collections.abc.Sequence[str]
    relation_ids: collections.abc.Sequence[str]


# Query


class KnowledgeChunk(pydantic.BaseModel):
    id: str
    text: str
    metadata: KnowledgeChunkMetadata


class KnowledgeChunkWithScore(KnowledgeChunk):
    score: float


class EntityResult(pydantic.BaseModel):
    name: str
    entity_type: str
    description: str | None = None


class RelationResult(pydantic.BaseModel):
    source_entity: str
    target_entity: str
    relation_type: str
    context: str | None = None


class QueryRequest(pydantic.BaseModel):
    query: str = pydantic.Field(min_length=1)
    filter: KnowledgeMetadataFilter | None = None
    top_k: int | None = pydantic.Field(default=5, gt=0, le=100)
    graph_depth: int | None = pydantic.Field(default=2, ge=0, le=5)


class QueryResponse(pydantic.BaseModel):
    query: str
    chunks: collections.abc.Sequence[KnowledgeChunkWithScore]
    entities: collections.abc.Sequence[EntityResult]
    relations: collections.abc.Sequence[RelationResult]


# Delete


class DeleteRequest(pydantic.BaseModel):
    filter: KnowledgeMetadataFilter


class DeleteResponse(pydantic.BaseModel):
    success: bool
