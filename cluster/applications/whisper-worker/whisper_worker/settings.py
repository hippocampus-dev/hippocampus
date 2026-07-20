import enum
import logging

import pydantic_settings
import sys


class WhisperModel(enum.StrEnum):
    TINY = "tiny"
    TINY_EN = "tiny.en"
    BASE = "base"
    BASE_EN = "base.en"
    SMALL = "small"
    SMALL_EN = "small.en"
    MEDIUM = "medium"
    MEDIUM_EN = "medium.en"
    LARGE_V1 = "large-v1"
    LARGE_V2 = "large-v2"
    LARGE_V3 = "large-v3"
    LARGE = "large"
    DISTIL_LARGE_V2 = "distil-large-v2"
    DISTIL_MEDIUM_EN = "distil-medium.en"
    DISTIL_SMALL_EN = "distil-small.en"
    DISTIL_LARGE_V3 = "distil-large-v3"


class Device(enum.StrEnum):
    CPU = "cpu"
    CUDA = "cuda"
    AUTO = "auto"


class Settings(pydantic_settings.BaseSettings):
    model_config = pydantic_settings.SettingsConfigDict(extra="allow", env_file=".env")

    log_level: str = "info"

    whisper_model: WhisperModel = WhisperModel.DISTIL_LARGE_V3
    device: Device = Device.AUTO

    langextract_api_key: str | None = None

    redis_host: str
    redis_port: int
    redis_key: str

    s3_endpoint_url: str | None = None
    s3_bucket: str = "whisper-worker"

    @staticmethod
    def is_debug() -> bool:
        return sys.prefix != sys.base_prefix

    def convert_log_level(self) -> int:
        return logging.getLevelName(self.log_level.upper())
