import logging

import pydantic


class Settings(pydantic.BaseSettings):
    log_level: str = "info"
    load_dotenv: bool = False

    embedding_retrieval_endpoint: str
    number_of_documents: int

    def convert_log_level(self) -> int:
        return logging.getLevelName(self.log_level.upper())

    class Config:
        env_file = ".env"
