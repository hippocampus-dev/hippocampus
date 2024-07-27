import enum
import logging

import pydantic


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


class Device(enum.StrEnum):
    CPU = "cpu"
    CUDA = "cuda"
    AUTO = "auto"


class Settings(pydantic.BaseSettings):
    log_level: str = "info"
    load_dotenv: bool = False

    whisper_model: WhisperModel = WhisperModel.LARGE
    device: Device = Device.AUTO

    redis_host: str
    redis_port: int
    redis_key: str

    s3_endpoint: str | None
    s3_bucket: str = "whisper-worker"

    def convert_log_level(self) -> int:
        return logging.getLevelName(self.log_level.upper())

    class Config:
        env_file = ".env"
