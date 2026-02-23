import asyncio
import collections.abc
import contextlib
import typing

import redis.asyncio

_STREAM_KEY_PREFIX = "sse:stream:"
_META_KEY_PREFIX = "sse:meta:"


class SSEBuffer:
    def __init__(self, client: redis.asyncio.Redis, ttl_seconds: int = 300):
        self._client = client
        self._ttl_seconds = ttl_seconds

    def _stream_key(self, stream_id: str) -> str:
        return f"{_STREAM_KEY_PREFIX}{stream_id}"

    def _meta_key(self, stream_id: str) -> str:
        return f"{_META_KEY_PREFIX}{stream_id}"

    async def open(self, stream_id: str) -> None:
        meta_key = self._meta_key(stream_id)
        async with self._client.pipeline(transaction=False) as pipe:
            pipe.hset(meta_key, "status", "running")
            pipe.expire(meta_key, self._ttl_seconds)
            await pipe.execute()

    @contextlib.asynccontextmanager
    async def produce(
        self, stream_id: str
    ) -> collections.abc.AsyncGenerator[None, None]:
        try:
            yield
        except asyncio.CancelledError:
            with contextlib.suppress(Exception):
                await self._mark_error(stream_id, "cancelled")
            raise
        except Exception as e:
            with contextlib.suppress(Exception):
                import cortex.exceptions

                await self._mark_error(
                    stream_id,
                    str(e),
                    retryable=isinstance(e, cortex.exceptions.RetryableError),
                )
            raise
        else:
            meta_key = self._meta_key(stream_id)
            async with self._client.pipeline(transaction=False) as pipe:
                pipe.hset(meta_key, "status", "complete")
                pipe.expire(meta_key, self._ttl_seconds)
                pipe.expire(self._stream_key(stream_id), self._ttl_seconds)
                await pipe.execute()

    async def append(
        self,
        stream_id: str,
        data: str,
        event: str | None = None,
    ) -> str:
        stream_key = self._stream_key(stream_id)
        fields: dict[str, str] = {"data": data}
        if event is not None:
            fields["event"] = event
        async with self._client.pipeline(transaction=False) as pipe:
            pipe.xadd(stream_key, fields)
            pipe.expire(stream_key, self._ttl_seconds)
            pipe.expire(self._meta_key(stream_id), self._ttl_seconds)
            results = await pipe.execute()
        entry_id = results[0]
        return entry_id.decode() if isinstance(entry_id, bytes) else entry_id

    async def events(
        self,
        stream_id: str,
        after_id: str = "0-0",
    ) -> collections.abc.AsyncGenerator[dict[str, typing.Any], None]:
        async for entry_id, fields in self._read_from(stream_id, after_id):
            sse_event: dict[str, typing.Any] = {
                "id": f"{stream_id}:{entry_id}",
                "data": fields["data"],
            }
            if "event" in fields:
                sse_event["event"] = fields["event"]
            yield sse_event

        status = await self.get_status(stream_id)
        if status == "error":
            meta_key = self._meta_key(stream_id)
            error, retryable = await self._client.hmget(meta_key, "error", "retryable")
            error_message = (
                error.decode()
                if isinstance(error, bytes)
                else (error or "unknown error")
            )
            is_retryable = (
                retryable.decode() if isinstance(retryable, bytes) else retryable
            ) == "1"
            if is_retryable:
                import cortex.exceptions

                raise cortex.exceptions.RetryableError(error_message)
            yield {
                "event": "error",
                "data": error_message,
            }

    async def _mark_error(
        self, stream_id: str, message: str, *, retryable: bool = False
    ) -> None:
        meta_key = self._meta_key(stream_id)
        mapping: dict[str, str] = {"status": "error", "error": message}
        if retryable:
            mapping["retryable"] = "1"
        async with self._client.pipeline(transaction=False) as pipe:
            pipe.hset(meta_key, mapping=mapping)
            pipe.expire(meta_key, self._ttl_seconds)
            pipe.expire(self._stream_key(stream_id), self._ttl_seconds)
            await pipe.execute()

    async def get_status(self, stream_id: str) -> str | None:
        meta_key = self._meta_key(stream_id)
        status = await self._client.hget(meta_key, "status")
        if status is None:
            return None
        return status.decode() if isinstance(status, bytes) else status

    async def _read_from(
        self,
        stream_id: str,
        after_id: str = "0-0",
    ) -> collections.abc.AsyncGenerator[tuple[str, dict[str, str]], None]:
        stream_key = self._stream_key(stream_id)

        last_id = after_id
        entries = await self._client.xrange(stream_key, min=f"({last_id}", max="+")
        for entry_id_raw, fields_raw in entries:
            entry_id = (
                entry_id_raw.decode()
                if isinstance(entry_id_raw, bytes)
                else entry_id_raw
            )
            fields = {
                (k.decode() if isinstance(k, bytes) else k): (
                    v.decode() if isinstance(v, bytes) else v
                )
                for k, v in fields_raw.items()
            }
            last_id = entry_id
            yield entry_id, fields

        while True:
            status = await self.get_status(stream_id)
            if status in ("complete", "error"):
                unread = await self._client.xrange(
                    stream_key, min=f"({last_id}", max="+", count=1
                )
                if not unread:
                    return
            if status is None and not await self._client.exists(stream_key):
                return

            result = await self._client.xread(
                {stream_key: last_id}, count=100, block=1000
            )
            if not result:
                status = await self.get_status(stream_id)
                if status in ("complete", "error"):
                    remaining = await self._client.xrange(
                        stream_key, min=f"({last_id}", max="+"
                    )
                    for entry_id_raw, fields_raw in remaining:
                        entry_id = (
                            entry_id_raw.decode()
                            if isinstance(entry_id_raw, bytes)
                            else entry_id_raw
                        )
                        fields = {
                            (k.decode() if isinstance(k, bytes) else k): (
                                v.decode() if isinstance(v, bytes) else v
                            )
                            for k, v in fields_raw.items()
                        }
                        last_id = entry_id
                        yield entry_id, fields
                    return
                if status is None and not await self._client.exists(stream_key):
                    return
                continue

            for _stream_name, messages in result:
                for entry_id_raw, fields_raw in messages:
                    entry_id = (
                        entry_id_raw.decode()
                        if isinstance(entry_id_raw, bytes)
                        else entry_id_raw
                    )
                    fields = {
                        (k.decode() if isinstance(k, bytes) else k): (
                            v.decode() if isinstance(v, bytes) else v
                        )
                        for k, v in fields_raw.items()
                    }
                    last_id = entry_id
                    yield entry_id, fields


def decode_event_id(event_id: str) -> tuple[str, str]:
    parts = event_id.split(":", 1)
    if len(parts) != 2 or not parts[0] or not parts[1]:
        raise ValueError(f"invalid event ID format: {event_id}")
    return parts[0], parts[1]
