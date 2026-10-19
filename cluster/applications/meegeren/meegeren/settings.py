import logging
import sys

import pydantic_settings

import meegeren.watermark


class Settings(pydantic_settings.BaseSettings):
    model_config = pydantic_settings.SettingsConfigDict(extra="allow", env_file=".env")

    host: str = "127.0.0.1"
    port: int = 8080
    log_level: str = "info"
    idle_timeout: int = 60 * 60 * 24
    termination_grace_period_seconds: int = 10

    device: str = "cuda"
    num_threads: int | None = None

    config_file: str = "/etc/meegeren/config.yaml"
    # config_file names models by paths relative to this, where `make download` puts them
    model_directory: str = "~/.cache/meegeren"
    pipeline: str | None = None
    upscaler: str | None = None
    watermark_font_path: str = "NotoSansCJKjp-Regular.otf"
    watermark_position: meegeren.watermark.Position = "bottom-right"

    # Covers outlines and shadows just past what the models mask
    mask_dilation: int = 15
    uploaded_mask_dilation: int = 0
    mask_feathering: int = 0
    max_upload_size: int = 32 * 1024 * 1024

    @staticmethod
    def is_debug() -> bool:
        return sys.prefix != sys.base_prefix

    def convert_log_level(self) -> int:
        return logging.getLevelName(self.log_level.upper())
