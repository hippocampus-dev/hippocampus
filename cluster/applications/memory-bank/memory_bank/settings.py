import enum
import logging
import typing

import pydantic_settings
import sys

OPENAI_VECTOR_SIZE = 1536


class EmbeddingModel(enum.StrEnum):
    ADA_V2 = "text-embedding-ada-002"
    ADA_V3_SMALL = "text-embedding-3-small"
    ADA_V3_LARGE = "text-embedding-3-large"

    # https://openai.com/pricing
    @property
    def price(self) -> float:
        match self:
            case EmbeddingModel.ADA_V2:
                return 0.0001 / 1000
            case EmbeddingModel.ADA_V3_SMALL:
                return 0.00002 / 1000
            case EmbeddingModel.ADA_V3_LARGE:
                return 0.00013 / 1000
            case _:
                raise ValueError(f"Unknown model: {self}")

    # https://platform.openai.com/docs/guides/embeddings/what-are-embeddings
    @property
    def max_tokens(self) -> int:
        match self:
            case (
                EmbeddingModel.ADA_V2
                | EmbeddingModel.ADA_V3_SMALL
                | EmbeddingModel.ADA_V3_LARGE
            ):
                return 8191
            case _:
                raise ValueError(f"Unknown model: {self}")


class DataStore(enum.StrEnum):
    Qdrant = "qdrant"


class Settings(pydantic_settings.BaseSettings):
    model_config = pydantic_settings.SettingsConfigDict(extra="allow", env_file=".env")

    host: str = "127.0.0.1"
    port: int = 8080
    log_level: str = "info"
    idle_timeout: int = 60 * 60 * 24
    termination_grace_period_seconds: int = 10

    default_chunk_size: int = 512
    embedding_batch_size: int = 32
    embedding_model: EmbeddingModel = EmbeddingModel.ADA_V3_SMALL

    datastore: DataStore = DataStore.Qdrant
    qdrant_host: str = "127.0.0.1"
    qdrant_port: int = 6333
    qdrant_timeout: int = 10
    qdrant_collection_name: str = "memory-bank"
    qdrant_replication_factor: int = 1
    qdrant_shard_number: int = 1

    categorizer_model: str = "gpt-5-mini"
    merger_model: str = "gpt-5-mini"
    service_tier: typing.Literal["auto", "default", "flex", "priority"] | None = None

    @staticmethod
    def is_debug() -> bool:
        return sys.prefix != sys.base_prefix

    def convert_log_level(self) -> int:
        return logging.getLevelName(self.log_level.upper())
