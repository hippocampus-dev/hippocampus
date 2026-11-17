import asyncio
import collections.abc
import json
import time
import uuid

import lance_graph
import lancedb
import opentelemetry.context
import pyarrow
import tiktoken

import lancer.datastore
import lancer.exceptions
import lancer.model
import lancer.settings
import lancer.telemetry


def _escape(value: str) -> str:
    return value.replace("'", "''")


class LanceDBDataStore(lancer.datastore.DataStore):
    UUID_NAMESPACE = uuid.UUID("e8a1b3c5-7d9f-4e2a-b6c8-1f3a5d7e9b0c")

    def __init__(
        self,
        default_chunk_size: int,
        embedding_batch_size: int,
        embedding_model: lancer.settings.EmbeddingModel,
        encoder: tiktoken.Encoding,
        entity_similarity_threshold: float,
        lancedb_path: str,
    ):
        super().__init__(
            default_chunk_size,
            embedding_batch_size,
            embedding_model,
            encoder,
            entity_similarity_threshold,
        )

        self.database = lancedb.connect(lancedb_path)
        self.knowledge_table = None
        self.entities_table = None
        self.relations_table = None
        self.mentions_table = None

    async def init(self):
        loop = asyncio.get_running_loop()
        context = opentelemetry.context.get_current()

        def setup(ctx):
            opentelemetry.context.attach(ctx)
            with lancer.telemetry.tracer.start_as_current_span("lancedb.init"):
                table_names = self.database.table_names()

                if "knowledge" in table_names:
                    self.knowledge_table = self.database.open_table("knowledge")
                if "entities" in table_names:
                    self.entities_table = self.database.open_table("entities")
                if "relations" in table_names:
                    self.relations_table = self.database.open_table("relations")
                if "mentions" in table_names:
                    self.mentions_table = self.database.open_table("mentions")

        await loop.run_in_executor(None, setup, context)

    def _ensure_knowledge_table(self):
        if self.knowledge_table is None:
            self.knowledge_table = self.database.create_table(
                "knowledge",
                schema=pyarrow.schema(
                    [
                        pyarrow.field("id", pyarrow.utf8()),
                        pyarrow.field("document_id", pyarrow.utf8()),
                        pyarrow.field("text", pyarrow.utf8()),
                        pyarrow.field("source", pyarrow.utf8()),
                        pyarrow.field("source_id", pyarrow.utf8()),
                        pyarrow.field("created_at", pyarrow.float64()),
                        pyarrow.field("updated_at", pyarrow.float64()),
                        pyarrow.field(
                            "vector",
                            pyarrow.list_(
                                pyarrow.float32(),
                                lancer.settings.OPENAI_VECTOR_SIZE,
                            ),
                        ),
                    ]
                ),
            )
        return self.knowledge_table

    def _ensure_entities_table(self):
        if self.entities_table is None:
            self.entities_table = self.database.create_table(
                "entities",
                schema=pyarrow.schema(
                    [
                        pyarrow.field("id", pyarrow.utf8()),
                        pyarrow.field("name", pyarrow.utf8()),
                        pyarrow.field("canonical_name", pyarrow.utf8()),
                        pyarrow.field("entity_type", pyarrow.utf8()),
                        pyarrow.field("description", pyarrow.utf8()),
                        pyarrow.field("metadata_json", pyarrow.utf8()),
                        pyarrow.field(
                            "vector",
                            pyarrow.list_(
                                pyarrow.float32(),
                                lancer.settings.OPENAI_VECTOR_SIZE,
                            ),
                        ),
                    ]
                ),
            )
        return self.entities_table

    def _ensure_relations_table(self):
        if self.relations_table is None:
            self.relations_table = self.database.create_table(
                "relations",
                schema=pyarrow.schema(
                    [
                        pyarrow.field("id", pyarrow.utf8()),
                        pyarrow.field("source_entity", pyarrow.utf8()),
                        pyarrow.field("target_entity", pyarrow.utf8()),
                        pyarrow.field("relation_type", pyarrow.utf8()),
                        pyarrow.field("context", pyarrow.utf8()),
                    ]
                ),
            )
        return self.relations_table

    def _ensure_mentions_table(self):
        if self.mentions_table is None:
            self.mentions_table = self.database.create_table(
                "mentions",
                schema=pyarrow.schema(
                    [
                        pyarrow.field("id", pyarrow.utf8()),
                        pyarrow.field("knowledge_chunk_id", pyarrow.utf8()),
                        pyarrow.field("entity_name", pyarrow.utf8()),
                        pyarrow.field("document_id", pyarrow.utf8()),
                    ]
                ),
            )
        return self.mentions_table

    async def _upsert_knowledge(
        self,
        chunks: collections.abc.Sequence[lancer.datastore.KnowledgeChunkWithEmbedding],
    ) -> collections.abc.Sequence[str]:
        loop = asyncio.get_running_loop()
        context = opentelemetry.context.get_current()
        now = time.time()

        def do_upsert(ctx):
            opentelemetry.context.attach(ctx)
            with lancer.telemetry.tracer.start_as_current_span(
                "lancedb.upsert_knowledge"
            ):
                table = self._ensure_knowledge_table()

                data = [
                    {
                        "id": chunk.id,
                        "document_id": chunk.metadata.document_id or "",
                        "text": chunk.text,
                        "source": chunk.metadata.source or "",
                        "source_id": chunk.metadata.source_id or "",
                        "created_at": chunk.metadata.created_at or now,
                        "updated_at": chunk.metadata.updated_at or now,
                        "vector": list(chunk.embedding),
                    }
                    for chunk in chunks
                ]

                document_id = chunks[0].metadata.document_id if chunks else None
                if document_id:
                    try:
                        table.delete(f"document_id = '{_escape(document_id)}'")
                    except Exception as e:
                        lancer.telemetry.logger.debug(
                            "delete before upsert failed: %s", e
                        )

                table.add(data)

                return [chunk.id for chunk in chunks]

        return await loop.run_in_executor(None, do_upsert, context)

    async def _upsert_entities(
        self,
        entities: collections.abc.Sequence[lancer.model.Entity],
        embeddings: collections.abc.Sequence[collections.abc.Sequence[float]],
        chunk_ids: collections.abc.Sequence[str],
    ) -> collections.abc.Sequence[str]:
        loop = asyncio.get_running_loop()
        context = opentelemetry.context.get_current()

        def do_upsert(ctx):
            opentelemetry.context.attach(ctx)
            with lancer.telemetry.tracer.start_as_current_span(
                "lancedb.upsert_entities"
            ):
                table = self._ensure_entities_table()
                mentions_table = self._ensure_mentions_table()

                entity_ids = []
                mention_records = []

                for entity, embedding in zip(entities, embeddings):
                    canonical_name = entity.name.strip().lower()
                    entity_type = entity.entity_type.strip().lower()

                    existing = None
                    try:
                        results = (
                            table.search()
                            .where(
                                f"canonical_name = '{_escape(canonical_name)}' "
                                f"AND entity_type = '{_escape(entity_type)}'"
                            )
                            .limit(1)
                            .to_list()
                        )
                        if results:
                            existing = results[0]
                    except Exception as e:
                        lancer.telemetry.logger.debug(
                            "entity exact match search failed: %s", e
                        )

                    if existing is None and len(table) > 0:
                        try:
                            results = table.search(list(embedding)).limit(1).to_list()
                            if results:
                                similarity = 1.0 - results[0]["_distance"]
                                result_type = results[0].get("entity_type", "")
                                if (
                                    similarity >= self.entity_similarity_threshold
                                    and result_type == entity_type
                                ):
                                    existing = results[0]
                        except Exception as e:
                            lancer.telemetry.logger.debug(
                                "entity similarity search failed: %s", e
                            )

                    if existing:
                        entity_id = existing["id"]
                        merged_description = existing.get("description", "") or ""
                        if entity.description and entity.description not in (
                            merged_description
                        ):
                            merged_description = (
                                f"{merged_description}\n{entity.description}".strip()
                            )

                        existing_metadata = {}
                        try:
                            existing_metadata = json.loads(
                                existing.get("metadata_json", "{}") or "{}"
                            )
                        except json.JSONDecodeError:
                            pass
                        if entity.metadata:
                            existing_metadata.update(entity.metadata)

                        table.update(
                            where=f"id = '{_escape(entity_id)}'",
                            values={
                                "name": entity.name,
                                "description": merged_description,
                                "metadata_json": json.dumps(existing_metadata),
                                "vector": list(embedding),
                            },
                        )
                    else:
                        entity_id = uuid.uuid5(
                            self.UUID_NAMESPACE,
                            f"{canonical_name}:{entity_type}",
                        ).hex

                        table.add(
                            [
                                {
                                    "id": entity_id,
                                    "name": entity.name,
                                    "canonical_name": canonical_name,
                                    "entity_type": entity_type,
                                    "description": entity.description or "",
                                    "metadata_json": json.dumps(entity.metadata or {}),
                                    "vector": list(embedding),
                                }
                            ]
                        )

                    entity_ids.append(entity_id)

                    for chunk_id in chunk_ids:
                        document_id = (
                            chunk_id.rsplit("_", 1)[0] if "_" in chunk_id else chunk_id
                        )
                        mention_records.append(
                            {
                                "id": uuid.uuid5(
                                    self.UUID_NAMESPACE,
                                    f"{chunk_id}:{canonical_name}",
                                ).hex,
                                "knowledge_chunk_id": chunk_id,
                                "entity_name": canonical_name,
                                "document_id": document_id,
                            }
                        )

                if mention_records:
                    mentions_table.add(mention_records)

                return entity_ids

        return await loop.run_in_executor(None, do_upsert, context)

    async def _upsert_relations(
        self,
        relations: collections.abc.Sequence[lancer.model.Relation],
    ) -> collections.abc.Sequence[str]:
        loop = asyncio.get_running_loop()
        context = opentelemetry.context.get_current()

        def do_upsert(ctx):
            opentelemetry.context.attach(ctx)
            with lancer.telemetry.tracer.start_as_current_span(
                "lancedb.upsert_relations"
            ):
                table = self._ensure_relations_table()

                data = []
                relation_ids = []

                for relation in relations:
                    relation_id = uuid.uuid5(
                        self.UUID_NAMESPACE,
                        f"{relation.source_entity}:{relation.target_entity}"
                        f":{relation.relation_type}",
                    ).hex

                    try:
                        table.delete(f"id = '{_escape(relation_id)}'")
                    except Exception as e:
                        lancer.telemetry.logger.debug(
                            "delete before relation upsert failed: %s", e
                        )

                    data.append(
                        {
                            "id": relation_id,
                            "source_entity": relation.source_entity,
                            "target_entity": relation.target_entity,
                            "relation_type": relation.relation_type,
                            "context": relation.context or "",
                        }
                    )
                    relation_ids.append(relation_id)

                if data:
                    table.add(data)

                return relation_ids

        return await loop.run_in_executor(None, do_upsert, context)

    async def _query(
        self,
        query: str,
        embedding: collections.abc.Sequence[float],
        metadata_filter: lancer.model.KnowledgeMetadataFilter | None,
        top_k: int,
        graph_depth: int,
    ) -> lancer.model.QueryResponse:
        loop = asyncio.get_running_loop()
        context = opentelemetry.context.get_current()

        def do_query(ctx):
            opentelemetry.context.attach(ctx)
            with lancer.telemetry.tracer.start_as_current_span("lancedb.query"):
                chunks = []
                entity_results = []
                relation_results = []

                if self.knowledge_table is not None:
                    search = self.knowledge_table.search(list(embedding)).limit(top_k)

                    if metadata_filter:
                        conditions = []
                        if metadata_filter.document_id:
                            conditions.append(
                                f"document_id = '{_escape(metadata_filter.document_id)}'"
                            )
                        if metadata_filter.source:
                            conditions.append(
                                f"source = '{_escape(metadata_filter.source)}'"
                            )
                        if metadata_filter.source_id:
                            conditions.append(
                                f"source_id = '{_escape(metadata_filter.source_id)}'"
                            )
                        if conditions:
                            search = search.where(" AND ".join(conditions))

                    results = search.to_list()

                    for result in results:
                        chunks.append(
                            lancer.model.KnowledgeChunkWithScore(
                                id=result["id"],
                                text=result["text"],
                                score=1.0 - result["_distance"],
                                metadata=lancer.model.KnowledgeChunkMetadata(
                                    document_id=result.get("document_id"),
                                    source=result.get("source") or None,
                                    source_id=result.get("source_id") or None,
                                    created_at=result.get("created_at"),
                                    updated_at=result.get("updated_at"),
                                ),
                            )
                        )

                seed_entity_names = set()

                if metadata_filter and metadata_filter.entity_name:
                    seed_entity_names.add(metadata_filter.entity_name.strip().lower())

                if self.entities_table is not None and len(self.entities_table) > 0:
                    try:
                        entity_search_results = (
                            self.entities_table.search(list(embedding))
                            .limit(3)
                            .to_list()
                        )
                        for result in entity_search_results:
                            similarity = 1.0 - result["_distance"]
                            if similarity >= 0.5:
                                seed_entity_names.add(result["canonical_name"])
                    except Exception as e:
                        lancer.telemetry.logger.debug(
                            "entity seed search failed: %s", e
                        )

                if (
                    seed_entity_names
                    and self.entities_table is not None
                    and self.relations_table is not None
                    and len(self.relations_table) > 0
                    and graph_depth > 0
                ):
                    entity_results, relation_results = self._expand_graph(
                        seed_entity_names, graph_depth
                    )

                return lancer.model.QueryResponse(
                    query=query,
                    chunks=chunks,
                    entities=entity_results,
                    relations=relation_results,
                )

        return await loop.run_in_executor(None, do_query, context)

    def _expand_graph(
        self,
        seed_entity_names: set[str],
        depth: int,
    ) -> tuple[
        collections.abc.Sequence[lancer.model.EntityResult],
        collections.abc.Sequence[lancer.model.RelationResult],
    ]:
        escaped_names = [_escape(n) for n in seed_entity_names]
        name_list = "', '".join(escaped_names)
        candidate_relations = (
            self.relations_table.search()
            .where(
                f"source_entity IN ('{name_list}') OR target_entity IN ('{name_list}')"
            )
            .limit(100)
            .to_list()
        )

        if not candidate_relations:
            entity_results = []
            for name in seed_entity_names:
                try:
                    results = (
                        self.entities_table.search()
                        .where(f"canonical_name = '{_escape(name)}'")
                        .limit(1)
                        .to_list()
                    )
                    if results:
                        entity_results.append(
                            lancer.model.EntityResult(
                                name=results[0]["name"],
                                entity_type=results[0]["entity_type"],
                                description=results[0].get("description") or None,
                            )
                        )
                except Exception as e:
                    lancer.telemetry.logger.debug(
                        "entity lookup failed for %s: %s", name, e
                    )
            return entity_results, []

        all_entity_names = set(seed_entity_names)
        for relation in candidate_relations:
            all_entity_names.add(relation["source_entity"])
            all_entity_names.add(relation["target_entity"])

        if depth >= 2:
            hop2_names = all_entity_names - seed_entity_names
            if hop2_names:
                escaped_hop2 = [_escape(n) for n in hop2_names]
                hop2_name_list = "', '".join(escaped_hop2)
                try:
                    hop2_relations = (
                        self.relations_table.search()
                        .where(
                            f"source_entity IN ('{hop2_name_list}') "
                            f"OR target_entity IN ('{hop2_name_list}')"
                        )
                        .limit(100)
                        .to_list()
                    )
                    for relation in hop2_relations:
                        all_entity_names.add(relation["source_entity"])
                        all_entity_names.add(relation["target_entity"])
                    candidate_relations.extend(hop2_relations)
                except Exception as e:
                    lancer.telemetry.logger.debug("hop2 expansion failed: %s", e)

        entities_arrow = None
        relations_arrow = None
        try:
            escaped_all = [_escape(n) for n in all_entity_names]
            entity_name_list = "', '".join(escaped_all)
            entity_rows = (
                self.entities_table.search()
                .where(f"canonical_name IN ('{entity_name_list}')")
                .limit(len(all_entity_names) + 10)
                .to_list()
            )

            if entity_rows:
                entities_arrow = pyarrow.table(
                    {
                        "name": [r["canonical_name"] for r in entity_rows],
                        "entity_type": [r["entity_type"] for r in entity_rows],
                        "description": [r.get("description", "") for r in entity_rows],
                    }
                )

            if candidate_relations:
                relations_arrow = pyarrow.table(
                    {
                        "source_entity": [
                            r["source_entity"] for r in candidate_relations
                        ],
                        "target_entity": [
                            r["target_entity"] for r in candidate_relations
                        ],
                        "relation_type": [
                            r["relation_type"] for r in candidate_relations
                        ],
                        "context": [r.get("context", "") for r in candidate_relations],
                    }
                )
        except Exception as e:
            lancer.telemetry.logger.debug("arrow table construction failed: %s", e)

        entity_results = []
        relation_results = []

        if entities_arrow is not None:
            config = (
                lance_graph.GraphConfigBuilder()
                .with_node_label("Entity", "name")
                .with_relationship("RELATES_TO", "source_entity", "target_entity")
                .build()
            )

            escaped_seeds = [_escape(n) for n in seed_entity_names]
            seed_list = "', '".join(escaped_seeds)
            cypher = (
                f"MATCH (a:Entity)-[r:RELATES_TO]-(b:Entity) "
                f"WHERE a.name IN ['{seed_list}'] "
                f"RETURN DISTINCT b.name, b.entity_type, b.description"
            )

            datasets = {"Entity": entities_arrow}
            if relations_arrow is not None:
                datasets["RELATES_TO"] = relations_arrow

            result = (
                lance_graph.CypherQuery(cypher).with_config(config).execute(datasets)
            )

            result_dict = result.to_pydict()
            for i in range(len(result_dict.get("b.name", []))):
                entity_results.append(
                    lancer.model.EntityResult(
                        name=result_dict["b.name"][i],
                        entity_type=result_dict.get("b.entity_type", [""])[i],
                        description=result_dict.get("b.description", [None])[i] or None,
                    )
                )

        seen = set()
        for relation in candidate_relations:
            key = (
                relation["source_entity"],
                relation["target_entity"],
                relation["relation_type"],
            )
            if key not in seen:
                seen.add(key)
                relation_results.append(
                    lancer.model.RelationResult(
                        source_entity=relation["source_entity"],
                        target_entity=relation["target_entity"],
                        relation_type=relation["relation_type"],
                        context=relation.get("context") or None,
                    )
                )

        return entity_results, relation_results

    async def _delete(
        self, metadata_filter: lancer.model.KnowledgeMetadataFilter
    ) -> bool:
        loop = asyncio.get_running_loop()
        context = opentelemetry.context.get_current()

        def do_delete(ctx):
            opentelemetry.context.attach(ctx)
            with lancer.telemetry.tracer.start_as_current_span("lancedb.delete"):
                if metadata_filter.document_id and self.knowledge_table is not None:
                    escaped_doc_id = _escape(metadata_filter.document_id)
                    self.knowledge_table.delete(f"document_id = '{escaped_doc_id}'")
                    if self.mentions_table is not None:
                        self.mentions_table.delete(f"document_id = '{escaped_doc_id}'")

                if metadata_filter.entity_name and self.entities_table is not None:
                    canonical_name = _escape(
                        metadata_filter.entity_name.strip().lower()
                    )
                    self.entities_table.delete(f"canonical_name = '{canonical_name}'")
                    if self.relations_table is not None:
                        self.relations_table.delete(
                            f"source_entity = '{canonical_name}' "
                            f"OR target_entity = '{canonical_name}'"
                        )
                    if self.mentions_table is not None:
                        self.mentions_table.delete(f"entity_name = '{canonical_name}'")

                if metadata_filter.source and self.knowledge_table is not None:
                    self.knowledge_table.delete(
                        f"source = '{_escape(metadata_filter.source)}'"
                    )

                if metadata_filter.source_id and self.knowledge_table is not None:
                    self.knowledge_table.delete(
                        f"source_id = '{_escape(metadata_filter.source_id)}'"
                    )

                return True

        return await loop.run_in_executor(None, do_delete, context)
