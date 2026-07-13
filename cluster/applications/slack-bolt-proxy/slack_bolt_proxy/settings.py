import logging

import pydantic_settings
import sys


class Settings(pydantic_settings.BaseSettings):
    model_config = pydantic_settings.SettingsConfigDict(extra="allow", env_file=".env")

    host: str = "127.0.0.1"
    port: int = 8080
    metrics_port: int = 8081
    log_level: str = "info"

    slack_app_token: str
    slack_bot_token: str
    slack_signing_secret: str
    slack_process_before_response: bool = False

    @staticmethod
    def is_debug() -> bool:
        return sys.prefix != sys.base_prefix

    def convert_log_level(self) -> int:
        return logging.getLevelName(self.log_level.upper())
