import collections.abc
import enum
import logging
import sys
import typing

import pydantic
import pydantic_settings

import cortex.llm.openai.agent
import cortex.llm.openai.model


class RateLimiterType(enum.StrEnum):
    Redis = "redis"


class BrainType(enum.StrEnum):
    S3 = "s3"


def inject(self: pydantic_settings.PydanticBaseSettingsSource):
    original_prepare_field_value = self.prepare_field_value

    def prepare_field_value(
        field_name: str,
        field: pydantic.fields.FieldInfo,
        value: typing.Any,
        value_is_complex: bool,
    ) -> typing.Any:
        match field_name:
            case (
                "allow_teams"
                | "allow_channels"
                | "allow_email_domains"
                | "slack_api_client_tokens"
            ):
                if not value:
                    return []
                return [x.strip() for x in value.split(",")]
        return original_prepare_field_value(field_name, field, value, value_is_complex)

    self.prepare_field_value = prepare_field_value


class Settings(pydantic_settings.BaseSettings):
    model_config = pydantic_settings.SettingsConfigDict(extra="allow", env_file=".env")

    host: str = "127.0.0.1"
    port: int = 8080
    metrics_port: int = 8081
    log_level: str = "info"
    idle_timeout: int = 60 * 60 * 24
    termination_grace_period_seconds: int = 10

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
    allow_email_domains: collections.abc.Sequence[str] = pydantic.Field(
        default_factory=list
    )
    slack_api_client_tokens: collections.abc.Sequence[str] = pydantic.Field(
        default_factory=list
    )

    disable_streaming: bool = False
    streaming_throttled_interval: int = 1
    system_prompt: str | None = None
    model: cortex.llm.openai.model.CompletionModel = (
        cortex.llm.openai.model.CompletionModel.GPT4O
    )
    embedding_model: cortex.llm.openai.model.EmbeddingModel = (
        cortex.llm.openai.model.EmbeddingModel.ADA_V3_SMALL
    )
    image_model: cortex.llm.openai.model.ImageModel = (
        cortex.llm.openai.model.ImageModel.x256
    )
    audio_model: cortex.llm.openai.model.AudioModel = (
        cortex.llm.openai.model.AudioModel.Whisper1
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

    brain_type: BrainType = BrainType.S3
    s3_endpoint_url: str | None = None
    s3_bucket: str = "cortex-bot"

    embedding_retrieval_url: str
    grafana_mcp_url: str
    playwright_mcp_url: str
    url_shortener_url: str
    chrome_devtools_protocol_url: str | None = None
    github_token: str
    google_client_id: str
    google_client_secret: str
    google_pre_issued_refresh_token: str
    google_custom_search_api_key: str | None = None
    google_custom_search_engine_id: str | None = None
    bing_subscription_key: str | None = None

    @staticmethod
    def is_debug() -> bool:
        return sys.prefix != sys.base_prefix

    def convert_log_level(self) -> int:
        return logging.getLevelName(self.log_level.upper())

    @classmethod
    def settings_customise_sources(
        cls,
        settings_cls: type[pydantic_settings.BaseSettings],
        init_settings: pydantic_settings.PydanticBaseSettingsSource,
        env_settings: pydantic_settings.PydanticBaseSettingsSource,
        dotenv_settings: pydantic_settings.PydanticBaseSettingsSource,
        file_secret_settings: pydantic_settings.PydanticBaseSettingsSource,
    ) -> tuple[pydantic_settings.PydanticBaseSettingsSource, ...]:
        inject(env_settings)
        inject(dotenv_settings)
        return init_settings, env_settings, dotenv_settings, file_secret_settings

