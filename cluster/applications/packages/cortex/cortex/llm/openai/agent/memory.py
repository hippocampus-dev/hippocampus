import collections.abc

import aredis_om

import cortex.llm.openai.model


class RedisMemory(aredis_om.JsonModel):
    context_id: str = aredis_om.Field(index=True)

    text: str
    embedding: collections.abc.Sequence[float] = aredis_om.Field(
        vector_options=aredis_om.VectorFieldOptions(
            algorithm=aredis_om.VectorFieldOptions.ALGORITHM.FLAT,
            type=aredis_om.VectorFieldOptions.TYPE.FLOAT64,
            dimension=cortex.llm.openai.model.OPENAI_VECTOR_SIZE,
            distance_metric=aredis_om.VectorFieldOptions.DISTANCE_METRIC.COSINE,
        ),
    )
    _embedding_score: float

    history: bytes
    created_at: float

    @property
    def similarity(self) -> float:
        # score is cosine distance, so we need to invert it
        return 1 - self._embedding_score

    class Meta:
        index_name = "memory"
