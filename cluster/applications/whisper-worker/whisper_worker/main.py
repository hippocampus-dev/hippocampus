import asyncio
import collections.abc
import datetime
import gzip
import io
import logging
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

if s.load_dotenv:
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


class Events(pydantic.BaseModel):
    __root__: collections.abc.Sequence[EventsItem]


async def main():
    redis_client = redis.asyncio.Redis(
        host=s.redis_host,
        port=s.redis_port,
        retry=redis.asyncio.retry.Retry(redis.backoff.ExponentialBackoff(), 3),
        retry_on_error=[redis.exceptions.ConnectionError, redis.exceptions.ReadOnlyError],
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

    s3_client = boto3.client("s3", endpoint_url=s.s3_endpoint)

    model = faster_whisper.WhisperModel(s.whisper_model, device=s.device)

    while True:
        events = await redis_client.lpop(s.redis_key)
        if events is None:
            await asyncio.sleep(1)
            continue

        e = Events.parse_raw(events)
        for event in e.__root__:
            for item in event.Event:
                try:
                    response = s3_client.get_object(Bucket=item.s3.bucket.name, Key=item.s3.object.key)
                except s3_client.exceptions.NoSuchKey:
                    continue

                body = response["Body"].read()

                segments, info = model.transcribe(io.BytesIO(body), vad_filter=True)
                if info.all_language_probs:
                    top_5 = sorted(info.all_language_probs, key=lambda x: x[1], reverse=True)[:5]
                    logging.info(f"Top 5 languages: {top_5}")

                results = ["[%.2fs -> %.2fs] %s" % (segment.start, segment.end, segment.text) for segment in segments]

                gzipped = io.BytesIO()
                with gzip.GzipFile(fileobj=gzipped, mode="w") as f:
                    f.write("\n".join(results).encode())

                s3_client.put_object(
                    Bucket=item.s3.bucket.name,
                    Key=f"out/{item.s3.object.key}.txt.gz",
                    Body=gzipped.getvalue(),
                )


if __name__ == "__main__":
    asyncio.run(main())
