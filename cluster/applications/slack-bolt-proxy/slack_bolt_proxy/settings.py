import logging

import pydantic


class Settings(pydantic.BaseSettings):
    host: str = "127.0.0.1"
    port: int = 8080
    metrics_port: int = 8081
    log_level: str = "info"
    load_dotenv: bool = False

    slack_app_token: str
    slack_bot_token: str
    slack_signing_secret: str
    slack_process_before_response: bool = False

    def convert_log_level(self) -> int:
        return logging.getLevelName(self.log_level.upper())

    class Config:
        env_file = ".env"
