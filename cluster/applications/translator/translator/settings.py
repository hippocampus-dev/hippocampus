import collections.abc
import enum
import logging
import typing

import pydantic
import pydantic_settings
import sys

import cortex.llm.openai.model


class RateLimiterType(enum.StrEnum):
    Redis = "redis"


class BrainType(enum.StrEnum):
    S3 = "s3"


def inject(self: pydantic_settings.PydanticBaseSettingsSource):
    original_prepare_field_value = self.prepare_field_value

    def prepare_field_value(
        field_name: str, field: pydantic.fields.FieldInfo, value: typing.Any, value_is_complex: bool
    ) -> typing.Any:
        match field_name:
            case "allow_teams" | "allow_channels" | "allow_email_domains" | "slack_api_client_tokens":
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
    allow_ext_shared_channel: bool = False
    allow_restricted_user: bool = False
    allow_teams: collections.abc.Sequence[str] = pydantic.Field(default_factory=list)
    allow_channels: collections.abc.Sequence[str] = pydantic.Field(default_factory=list)
    allow_email_domains: collections.abc.Sequence[str] = pydantic.Field(default_factory=list)
    slack_api_client_tokens: collections.abc.Sequence[str] = pydantic.Field(default_factory=list)

    model: cortex.llm.openai.model.CompletionModel = cortex.llm.openai.model.CompletionModel.GPT4O

    redis_host: str
    redis_port: int
    rate_limiter_type: RateLimiterType = RateLimiterType.Redis
    rate_limit_per_interval: int = 50000
    rate_limit_interval_seconds: int = 3600

    brain_type: BrainType = BrainType.S3
    s3_endpoint_url: str | None = None
    s3_bucket: str = "translator"

    url_shortener_url: str

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
