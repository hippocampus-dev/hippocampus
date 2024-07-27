import collections.abc
import enum
import logging
import typing

import pydantic

import cortex.llm.openai.agent
import cortex.llm.openai.model


class RateLimiterType(enum.StrEnum):
    Redis = "redis"


class BrainType(enum.StrEnum):
    S3 = "s3"


class Settings(pydantic.BaseSettings):
    host: str = "127.0.0.1"
    port: int = 8080
    metrics_port: int = 8081
    log_level: str = "info"
    idle_timeout: int = 60 * 60 * 24
    termination_grace_period_seconds: int = 10
    reload: bool = False
    access_log: bool = False
    load_dotenv: bool = False

    slack_app_token: str = ""
    slack_bot_token: str
    slack_signing_secret: str
    slack_process_before_response: bool = False
    slack_bot_member_id: str
    callback_reaction: str | None = None
    allow_ext_shared_channel: bool = False
    allow_restricted_user: bool = False
    allow_teams: collections.abc.Sequence[str] = pydantic.Field(default_factory=list)
    allow_channels: collections.abc.Sequence[str] = pydantic.Field(default_factory=list)
    allow_email_domains: collections.abc.Sequence[str] = pydantic.Field(default_factory=list)
    slack_api_client_tokens: collections.abc.Sequence[str] = pydantic.Field(default_factory=list)

    disable_streaming: bool = False
    streaming_throttled_interval: int = 1
    system_prompt: str | None = None
    model: cortex.llm.openai.model.CompletionModel = cortex.llm.openai.model.CompletionModel.GPT4O
    embedding_model: cortex.llm.openai.model.EmbeddingModel = cortex.llm.openai.model.EmbeddingModel.ADA_V3_SMALL
    image_model: cortex.llm.openai.model.ImageModel = cortex.llm.openai.model.ImageModel.Standard1024x1024
    audio_model: cortex.llm.openai.model.AudioModel = cortex.llm.openai.model.AudioModel.Whisper1
    loop_budget: int = 10

    memory_type: cortex.llm.openai.agent.MemoryType = cortex.llm.openai.agent.MemoryType.Redis
    redis_host: str
    redis_port: int
    rate_limiter_type: RateLimiterType = RateLimiterType.Redis
    rate_limit_per_interval: int = 50000
    rate_limit_interval_seconds: int = 3600

    brain_type: BrainType = BrainType.S3
    s3_endpoint: str | None
    s3_bucket: str = "cortex-bot"

    embedding_retrieval_endpoint: str
    github_token: str
    google_client_id: str
    google_client_secret: str
    google_pre_issued_refresh_token: str
    bing_subscription_key: str | None = None

    def convert_log_level(self) -> int:
        return logging.getLevelName(self.log_level.upper())

    class Config:
        env_file = ".env"

        @classmethod
        def parse_env_var(cls, field_name: str, raw_val: str) -> typing.Any:
            match field_name:
                case "allow_teams" | "allow_channels" | "allow_email_domains" | "slack_api_client_tokens":
                    return [x.strip() for x in raw_val.split(",")]
            return cls.json_loads(raw_val)  # noqa
