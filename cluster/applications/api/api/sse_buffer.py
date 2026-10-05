import asyncio
import base64
import collections.abc
import contextlib
import os
import typing

import cryptography.hazmat.primitives.ciphers.aead
import redis.asyncio

_STREAM_KEY_PREFIX = "sse:stream:"
_META_KEY_PREFIX = "sse:meta:"

_KEY_SIZE = 32
_NONCE_SIZE = 12


def _derive_key(secret: str) -> bytes:
    return base64.urlsafe_b64decode(secret)


def _encrypt(key: bytes, plaintext: str) -> str:
    nonce = os.urandom(_NONCE_SIZE)
    aesgcm = cryptography.hazmat.primitives.ciphers.aead.AESGCM(key)
    ciphertext = aesgcm.encrypt(nonce, plaintext.encode(), None)
    return base64.b64encode(nonce + ciphertext).decode()


def _decrypt(key: bytes, token: str) -> str:
    raw = base64.b64decode(token)
    nonce = raw[:_NONCE_SIZE]
    ciphertext = raw[_NONCE_SIZE:]
    aesgcm = cryptography.hazmat.primitives.ciphers.aead.AESGCM(key)
    return aesgcm.decrypt(nonce, ciphertext, None).decode()


class SSEBuffer:
    client: redis.asyncio.Redis
    ttl_seconds: int

    def __init__(self, client: redis.asyncio.Redis, ttl_seconds: int = 300):
        self.client = client
        self.ttl_seconds = ttl_seconds

    def _stream_key(self, stream_id: str) -> str:
        return f"{_STREAM_KEY_PREFIX}{stream_id}"

    def _meta_key(self, stream_id: str) -> str:
        return f"{_META_KEY_PREFIX}{stream_id}"

    async def open(self, stream_id: str) -> str:
        secret = base64.urlsafe_b64encode(os.urandom(_KEY_SIZE)).decode()
        meta_key = self._meta_key(stream_id)
        async with self.client.pipeline(transaction=False) as pipe:
            pipe.hset(meta_key, mapping={"status": "running"})
            pipe.expire(meta_key, self.ttl_seconds)
            await pipe.execute()
        return secret

    @contextlib.asynccontextmanager
    async def produce(
        self, stream_id: str, secret: str
    ) -> collections.abc.AsyncGenerator[None, None]:
        try:
            yield
        except asyncio.CancelledError:
            with contextlib.suppress(Exception):
                await self._mark_error(stream_id, secret, "cancelled")
            raise
        except Exception as e:
            with contextlib.suppress(Exception):
                import cortex.exceptions

                await self._mark_error(
                    stream_id,
                    secret,
                    str(e),
                    retryable=isinstance(e, cortex.exceptions.RetryableError),
                )
            raise
        else:
            try:
                meta_key = self._meta_key(stream_id)
                async with self.client.pipeline(transaction=False) as pipe:
                    pipe.hset(meta_key, "status", "complete")
                    pipe.expire(meta_key, self.ttl_seconds)
                    pipe.expire(self._stream_key(stream_id), self.ttl_seconds)
                    await pipe.execute()
            except Exception:
                with contextlib.suppress(Exception):
                    await self._mark_error(
                        stream_id, secret, "failed to mark complete", retryable=True
                    )

    async def append(
        self,
        stream_id: str,
        secret: str,
        data: str,
        event: str | None = None,
    ) -> str:
        key = _derive_key(secret)
        encrypted_data = _encrypt(key, data)

        stream_key = self._stream_key(stream_id)
        fields: dict[str, str] = {"data": encrypted_data}
        if event is not None:
            fields["event"] = event
        async with self.client.pipeline(transaction=False) as pipe:
            pipe.xadd(stream_key, fields)
            pipe.expire(stream_key, self.ttl_seconds)
            pipe.expire(self._meta_key(stream_id), self.ttl_seconds)
            results = await pipe.execute()
        return results[0]

    async def events(
        self,
        stream_id: str,
        secret: str,
        after_id: str = "0-0",
    ) -> collections.abc.AsyncGenerator[dict[str, typing.Any], None]:
        key = _derive_key(secret)

        async for entry_id, fields in self._read_from(stream_id, after_id):
            sse_event: dict[str, typing.Any] = {
                "id": encode_event_id(stream_id, secret, entry_id),
                "data": _decrypt(key, fields["data"]),
            }
            if "event" in fields:
                sse_event["event"] = fields["event"]
            yield sse_event

    async def _mark_error(
        self, stream_id: str, secret: str, message: str, *, retryable: bool = False
    ) -> None:
        event = "retryable_error" if retryable else "error"
        await self.append(stream_id, secret, message, event=event)

        meta_key = self._meta_key(stream_id)
        async with self.client.pipeline(transaction=False) as pipe:
            pipe.hset(meta_key, "status", "error")
            pipe.expire(meta_key, self.ttl_seconds)
            await pipe.execute()

    async def get_status(self, stream_id: str) -> str | None:
        return await self.client.hget(self._meta_key(stream_id), "status")

    async def _read_from(
        self,
        stream_id: str,
        after_id: str = "0-0",
    ) -> collections.abc.AsyncGenerator[tuple[str, dict[str, str]], None]:
        stream_key = self._stream_key(stream_id)
        last_id = after_id

        while True:
            result = await self.client.xread(
                {stream_key: last_id}, count=100, block=1000
            )
            if result:
                for _stream_name, messages in result:
                    for entry_id, fields in messages:
                        last_id = entry_id
                        yield entry_id, fields

            status = await self.get_status(stream_id)
            if status in ("complete", "error"):
                remaining = await self.client.xrange(
                    stream_key, min=f"({last_id}", max="+"
                )
                for entry_id, fields in remaining:
                    last_id = entry_id
                    yield entry_id, fields
                return
            if status is None and not await self.client.exists(stream_key):
                return


def encode_event_id(stream_id: str, secret: str, entry_id: str) -> str:
    return f"{stream_id}:{secret}:{entry_id}"


def decode_event_id(event_id: str) -> tuple[str, str, str]:
    parts = event_id.split(":", 2)
    if len(parts) != 3 or not parts[0] or not parts[1] or not parts[2]:
        raise ValueError(f"invalid event ID format: {event_id}")
    return parts[0], parts[1], parts[2]
