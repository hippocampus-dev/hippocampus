import enum
import logging

import pydantic_settings
import sys

import cortex.llm.openai.agent
import cortex.llm.openai.model


class RateLimiterType(enum.StrEnum):
    Redis = "redis"


class Settings(pydantic_settings.BaseSettings):
    model_config = pydantic_settings.SettingsConfigDict(extra="allow", env_file=".env")

    host: str = "127.0.0.1"
    port: int = 8080
    log_level: str = "info"
    idle_timeout: int = 60 * 60 * 24
    termination_grace_period_seconds: int = 10

    system_prompt: str | None = None
    model: cortex.llm.openai.model.CompletionModel = (
        cortex.llm.openai.model.CompletionModel.GPT4O
    )
    embedding_model: cortex.llm.openai.model.EmbeddingModel = (
        cortex.llm.openai.model.EmbeddingModel.ADA_V3_SMALL
    )
    loop_budget: int = 10

    memory_type: cortex.llm.openai.agent.MemoryType = (
        cortex.llm.openai.agent.MemoryType.Redis
    )
    redis_host: str
    redis_port: int
    rate_limiter_type: RateLimiterType = RateLimiterType.Redis
    rate_limit_per_interval: int = 100_000
    rate_limit_interval_seconds: int = 3600

    chrome_devtools_protocol_url: str | None = None
    github_token: str
    slack_bot_token: str
    google_client_id: str
    google_client_secret: str
    google_pre_issued_refresh_token: str

    @staticmethod
    def is_debug() -> bool:
        return sys.prefix != sys.base_prefix

    def convert_log_level(self) -> int:
        return logging.getLevelName(self.log_level.upper())
