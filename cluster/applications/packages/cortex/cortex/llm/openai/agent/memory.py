import collections.abc

import aredis_om

import cortex.llm.openai.model


class RedisMemory(aredis_om.JsonModel, index=True):
    context_id: str = aredis_om.Field(index=True)

    text: str
    embedding: list[float] = aredis_om.Field(
        vector_options=aredis_om.VectorFieldOptions(
            algorithm=aredis_om.VectorFieldOptions.ALGORITHM.FLAT,
            type=aredis_om.VectorFieldOptions.TYPE.FLOAT64,
            dimension=cortex.llm.openai.model.OPENAI_VECTOR_SIZE,
            distance_metric=aredis_om.VectorFieldOptions.DISTANCE_METRIC.COSINE,
        ),
    )
    embedding_score: float | None = None

    history: bytes
    created_at: float

    @property
    def similarity(self) -> float:
        if self.embedding_score is None:
            return 0.0

        # score is cosine distance, so we need to invert it
        return 1 - self.embedding_score

    class Meta:
        index_name = "memory"
