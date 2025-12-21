import asyncio
import collections.abc
import datetime
import gzip
import io
import logging
import os
import tempfile
import typing

import boto3
import dotenv
import faster_whisper
import pydantic
import pythonjsonlogger.jsonlogger
import redis.asyncio
import redis.asyncio.retry

import whisper_worker.settings

s = whisper_worker.settings.Settings()

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
    (
        handler.setFormatter(
            logging.Formatter(
                "time=%(asctime)s name=%(name)s severitytext=%(levelname)s body=%(message)s"
            )
        ),
    )
logging.basicConfig(level=s.convert_log_level(), handlers=[handler])


class Bucket(pydantic.BaseModel):
    name: str


class Object(pydantic.BaseModel):
    key: str


class S3(pydantic.BaseModel):
    bucket: Bucket
    object: Object


class EventItem(pydantic.BaseModel):
    s3: S3


class EventsItem(pydantic.BaseModel):
    Event: collections.abc.Sequence[EventItem]


class Events(pydantic.RootModel[collections.abc.Sequence[EventsItem]]):
    pass


async def main():
    redis_client = redis.asyncio.Redis(
        host=s.redis_host,
        port=s.redis_port,
        retry=redis.asyncio.retry.Retry(redis.backoff.ExponentialBackoff(), 3),
        retry_on_error=[
            redis.exceptions.ConnectionError,
            redis.exceptions.ReadOnlyError,
        ],
    )
    # HACK: execute_command ignores auto_close_connection_pool
    import types

    async def new_execute_command(self, *args, **options):
        try:
            return await redis.asyncio.Redis.execute_command(self, *args, **options)
        finally:
            if self.auto_close_connection_pool:
                await self.connection_pool.disconnect()

    redis_client.execute_command = types.MethodType(new_execute_command, redis_client)

    s3_client = boto3.client("s3", endpoint_url=s.s3_endpoint_url)

    model = faster_whisper.WhisperModel(
        s.whisper_model, device=s.device, local_files_only=False
    )

    while True:
        _, events = await redis_client.blpop(s.redis_key)
        e = Events.model_validate_json(events)
        for event in e.root:
            for item in event.Event:
                try:
                    response = s3_client.get_object(
                        Bucket=item.s3.bucket.name, Key=item.s3.object.key
                    )
                except s3_client.exceptions.NoSuchKey:
                    continue

                body = response["Body"].read()

                segments, info = model.transcribe(io.BytesIO(body), vad_filter=True)
                if info.all_language_probs:
                    top_5 = sorted(
                        info.all_language_probs, key=lambda x: x[1], reverse=True
                    )[:5]
                    logging.info(f"Top 5 languages: {top_5}")

                results = [
                    "[%.2fs -> %.2fs] %s" % (segment.start, segment.end, segment.text)
                    for segment in segments
                ]

                gzipped = io.BytesIO()
                with gzip.GzipFile(fileobj=gzipped, mode="w") as f:
                    f.write("\n".join(results).encode())

                s3_client.put_object(
                    Bucket=item.s3.bucket.name,
                    Key=f"out/{item.s3.object.key}.txt.gz",
                    Body=gzipped.getvalue(),
                    Metadata=response["Metadata"],
                )

                if s.langextract_api_key:
                    import langextract

                    result = langextract.extract(
                        text_or_documents="\n".join([segment.text for segment in segments]),
                        examples=[
                            langextract.data.ExampleData(
                                text="田中: はい、それでは本日の会議を始めさせていただきます。まず最初に、前回の議事録の確認から行いたいと思います。佐藤さん、お願いできますか？\n佐藤: はい、承知しました。前回は新製品のマーケティング戦略について話し合いました。",
                                extractions=[
                                    langextract.data.Extraction(
                                        extraction_class="topic",
                                        extraction_text="前回の議事録の確認",
                                    ),
                                    langextract.data.Extraction(
                                        extraction_class="topic",
                                        extraction_text="新製品のマーケティング戦略",
                                    ),
                                    langextract.data.Extraction(
                                        extraction_class="action_item",
                                        extraction_text="佐藤さん、お願いできますか",
                                        attributes={"type": "request", "assignee": "佐藤", "status": "pending"}
                                    ),
                                ]
                            ),
                            langextract.data.ExampleData(
                                text="[Interviewer] So, can you tell me about your experience with cloud infrastructure?\n[Candidate] Sure! I've been working with AWS for about three years now. I've designed and implemented scalable architectures using services like EC2, S3, and Lambda.\n[Interviewer] That's great. What about container orchestration?",
                                extractions=[
                                    langextract.data.Extraction(
                                        extraction_class="topic",
                                        extraction_text="cloud infrastructure",
                                    ),
                                    langextract.data.Extraction(
                                        extraction_class="topic",
                                        extraction_text="AWS",
                                    ),
                                    langextract.data.Extraction(
                                        extraction_class="topic",
                                        extraction_text="container orchestration",
                                    ),
                                    langextract.data.Extraction(
                                        extraction_class="action_item",
                                        extraction_text="tell me about your experience",
                                        attributes={"type": "request", "assignee": "Candidate", "status": "completed"}
                                    ),
                                ]
                            ),
                        ],
                        model_id="gemini-2.5-flash",
                    )

                    with tempfile.TemporaryDirectory() as d:
                        output_file = "data.jsonl"
                        langextract.io.save_annotated_documents(
                            [result],
                            output_name=output_file,
                            output_dir=d,
                        )

                        output_path = os.path.join(d, output_file)
                        with open(output_path, "rb") as f:
                            s3_client.put_object(
                                Bucket=item.s3.bucket.name,
                                Key=f"out/{item.s3.object.key}.extractions.jsonl",
                                Body=f.read(),
                                Metadata=response["Metadata"],
                            )


if __name__ == "__main__":
    asyncio.run(main())
