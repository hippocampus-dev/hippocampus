import collections.abc
import contextlib
import enum
import time
import uuid

import redis.asyncio


class CircuitBreakerFailure(Exception):
    pass


class CircuitBreakerOpen(Exception):
    pass


class State(enum.Enum):
    CLOSED = "closed"
    OPEN = "open"
    HALF_OPEN = "half-open"


class RedisCircuitBreaker:
    client: redis.asyncio.Redis
    name: str
    failure_threshold: int
    reset_timeout_seconds: int
    evaluation_window_seconds: int
    key_failures: str
    key_opened_at: str
    key_singleflight: str

    def __init__(
        self,
        client: redis.asyncio.Redis,
        name: str,
        failure_threshold: int = 3,
        reset_timeout_seconds: int = 60,
        evaluation_window_seconds: int = 120,
    ) -> None:
        self.client = client
        self.name = name
        self.failure_threshold = failure_threshold
        self.reset_timeout_seconds = reset_timeout_seconds
        self.evaluation_window_seconds = evaluation_window_seconds

        self.key_opened_at = f"circuit_breaker:{name}:opened_at"
        self.key_failures = f"circuit_breaker:{name}:failures"
        self.key_singleflight = f"circuit_breaker:{name}:singleflight"

    @contextlib.asynccontextmanager
    async def call(self) -> collections.abc.AsyncGenerator[None, None]:
        state = await self._state()

        if state is State.OPEN:
            raise CircuitBreakerOpen()

        if state is State.HALF_OPEN:
            acquired = await self.client.set(
                self.key_singleflight, "1", nx=True, ex=self.reset_timeout_seconds
            )
            if not acquired:
                raise CircuitBreakerOpen()

        try:
            yield
        except CircuitBreakerFailure as e:
            await self._on_failure()
            raise e.__cause__ from None
        finally:
            if state is State.HALF_OPEN:
                await self.client.delete(self.key_singleflight)

    async def _state(self) -> State:
        raw = await self.client.get(self.key_opened_at)
        if raw is None:
            return State.CLOSED

        elapsed = time.time() - float(raw)
        if elapsed < self.reset_timeout_seconds:
            return State.OPEN
        return State.HALF_OPEN

    async def _on_failure(self) -> None:
        now = time.time()
        pipe = self.client.pipeline(transaction=False)
        pipe.zadd(self.key_failures, {uuid.uuid4().hex: now})
        pipe.zremrangebyscore(
            self.key_failures, "-inf", now - self.evaluation_window_seconds
        )
        pipe.zcard(self.key_failures)
        pipe.expire(self.key_failures, self.evaluation_window_seconds)
        results = await pipe.execute()
        count = results[2]

        if count >= self.failure_threshold:
            await self.client.set(
                self.key_opened_at,
                str(now),
                ex=self.reset_timeout_seconds,
            )
