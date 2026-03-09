import asyncio
import datetime
import logging
import typing

import aiohttp
import aiohttp.client_exceptions
import dotenv
import pythonjsonlogger.jsonlogger
import tensorflow_datasets

import embedding_retrieval.model
import embedding_retrieval_loader.settings

s = embedding_retrieval_loader.settings.Settings()

if s.is_debug():
    dotenv.load_dotenv(override=True)


class JsonFormatter(pythonjsonlogger.jsonlogger.JsonFormatter):
    def add_fields(
        self,
        log_record: dict[str, typing.Any],
        record: logging.LogRecord,
        message_dict: dict[str, typing.Any],
    ):
        now = datetime.datetime.now()
        log_record["name"] = record.name
        log_record["time"] = now.isoformat()
        log_record["severitytext"] = record.levelname

        super().add_fields(log_record, record, message_dict)


handler = logging.StreamHandler()
handler.setFormatter(JsonFormatter())
if s.is_debug():
    handler.setFormatter(logging.Formatter(
        "time=%(asctime)s name=%(name)s severitytext=%(levelname)s body=%(message)s"
    )),
logging.basicConfig(level=s.convert_log_level(), handlers=[handler])


async def upsert(request: embedding_retrieval.model.UpsertRequest) -> embedding_retrieval.model.UpsertResponse:
    max_retries = 3
    retries = 0
    while retries < max_retries:
        try:
            async with aiohttp.ClientSession(trust_env=True) as session:
                async with session.post(
                    f"{s.embedding_retrieval_url}/upsert",
                    json=request.model_dump(),
                ) as response:
                    if response.status != 200:
                        match response.status:
                            case (409 | 429 | 502 | 503 | 504):
                                if retries < max_retries:
                                    retries += 1
                                    continue
                        raise ValueError(f"invalid response status: {response.status}")
                    return embedding_retrieval.model.UpsertResponse.model_validate(await response.json())
        except (
                aiohttp.ClientConnectionError,  # ECONNREFUSED, EPIPE, ECONNRESET
                aiohttp.client_exceptions.ServerDisconnectedError,
                asyncio.TimeoutError,
        ) as e:
            retries += 1
            if retries < max_retries:
                continue
            raise e


async def main():
    articles = {}

    ds = tensorflow_datasets.load("wiki40b/ja", split="train")
    for d in ds.take(s.number_of_documents):
        text = d["text"].numpy().decode("utf-8")
        lines = text.split("\n")

        i = 0
        current_article = None
        current_section = None
        while i < len(lines):
            line = lines[i]
            match line:
                case "_START_ARTICLE_":
                    i += 1
                    current_article = lines[i]
                    articles[current_article] = {}
                case "_START_SECTION_":
                    i += 1
                    current_section = lines[i]
                case "_START_PARAGRAPH_":
                    i += 1
                    paragraph = lines[i]
                    if current_section is None:
                        current_section = "要約"
                    articles[current_article][current_section] = paragraph.replace("_NEWLINE_", "\n")
                case _:
                    i += 1

    now = datetime.datetime.now().timestamp()
    response = await upsert(request=embedding_retrieval.model.UpsertRequest(
        documents=[
            embedding_retrieval.model.Document(
                id=f"https://ja.wikipedia.org/wiki/{title}",
                text="\n".join(article.values()),
                metadata=embedding_retrieval.model.DocumentMetadata(
                    source="wikipedia",
                    source_id=f"https://ja.wikipedia.org/wiki/{title}",
                    created_at=now,
                    updated_at=now,
                ),
            )
            for title, article in articles.items()
        ],
    ))
    print(response.ids)


if __name__ == "__main__":
    asyncio.run(main())
