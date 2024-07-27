import logging

import pydantic


class Settings(pydantic.BaseSettings):
    host: str = "127.0.0.1"
    port: int = 8080
    log_level: str = "info"
    idle_timeout: int = 60 * 60 * 24
    termination_grace_period_seconds: int = 10
    reload: bool = False
    access_log: bool = False
    load_dotenv: bool = False

    s3_endpoint: str | None
    s3_bucket: str = "embedding-gateway"

    def convert_log_level(self) -> int:
        return logging.getLevelName(self.log_level.upper())

    class Config:
        env_file = ".env"
