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


def inject(self: pydantic_settings.PydanticBaseSettingsSource):
    original_prepare_field_value = self.prepare_field_value

    def prepare_field_value(
        field_name: str,
        field: pydantic.fields.FieldInfo,
        value: typing.Any,
        value_is_complex: bool,
    ) -> typing.Any:
        match field_name:
            case "allowed_origins":
                if not value:
                    return []
                return [x.strip() for x in value.split(",")]
        return original_prepare_field_value(field_name, field, value, value_is_complex)

    self.prepare_field_value = prepare_field_value


class Settings(pydantic_settings.BaseSettings):
    model_config = pydantic_settings.SettingsConfigDict(extra="allow", env_file=".env")

    host: str = "127.0.0.1"
    port: int = 8080
    log_level: str = "info"
    idle_timeout: int = 60 * 60 * 24
    termination_grace_period_seconds: int = 10

    system_prompt: str | None = None
    model: cortex.llm.openai.model.CompletionModel = (
        cortex.llm.openai.model.CompletionModel.GPT_6_SOL
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

    sse_buffer_ttl_seconds: int = 300

    realtime_group_window_seconds: int = 10
    realtime_group_backlog_seconds: int = 30
    realtime_group_match_threshold: float = 0.7
    realtime_group_sharpness_threshold: float = 0.35
    realtime_group_recheck_seconds: int = 1
    allowed_origins: collections.abc.Sequence[str] = pydantic.Field(
        default_factory=list
    )

    flex_circuit_breaker_failure_threshold: int = 3
    flex_circuit_breaker_reset_timeout_seconds: int = 60
    flex_circuit_breaker_evaluation_window_seconds: int = 120
    flex_circuit_breaker_latency_rate_threshold: float = 0.5
    flex_circuit_breaker_latency_z_score_threshold: float = 2.0
    flex_circuit_breaker_latency_min_samples: int = 10
    flex_circuit_breaker_fallback_tier: str = "auto"

    chrome_devtools_protocol_url: str | None = None
    browser_proxy_url: str | None = None
    browser_proxy_certificate_authority_file: str | None = None
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
