import collections.abc
import contextlib
import time

import redis.asyncio


class CircuitBreakerFailure(Exception):
    pass


class CircuitBreakerOpen(Exception):
    pass


class RedisCircuitBreaker:
    client: redis.asyncio.Redis
    name: str
    failure_threshold: int
    reset_timeout_seconds: int
    evaluation_window_seconds: int
    key_failures: str
    key_opened_at: str

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

        self.key_failures = f"circuit_breaker:{name}:failures"
        self.key_opened_at = f"circuit_breaker:{name}:opened_at"

    @contextlib.asynccontextmanager
    async def call(self) -> collections.abc.AsyncGenerator[None, None]:
        opened_at = await self._get_opened_at()
        if self._is_open(opened_at):
            raise CircuitBreakerOpen()

        try:
            yield
        except CircuitBreakerFailure:
            await self._on_failure()
            raise
        else:
            await self._on_success(opened_at)

    async def _get_opened_at(self) -> float | None:
        raw = await self.client.get(self.key_opened_at)
        if raw is None:
            return None
        return float(raw)

    def _is_open(self, opened_at: float | None) -> bool:
        if opened_at is None:
            return False

        elapsed = time.time() - opened_at
        if elapsed >= self.reset_timeout_seconds:
            return False

        return True

    async def _on_failure(self) -> None:
        now = time.time()
        pipe = self.client.pipeline(transaction=False)
        pipe.zadd(self.key_failures, {str(now): now})
        pipe.zremrangebyscore(
            self.key_failures, "-inf", now - self.evaluation_window_seconds
        )
        pipe.zcard(self.key_failures)
        results = await pipe.execute()
        count = results[2]

        if count >= self.failure_threshold:
            await self.client.set(
                self.key_opened_at,
                str(now),
                ex=self.reset_timeout_seconds,
            )

    async def _on_success(self, opened_at_before_call: float | None) -> None:
        if opened_at_before_call is None:
            return

        opened_at_now = await self._get_opened_at()
        if opened_at_now is None:
            return

        if opened_at_now != opened_at_before_call:
            return

        elapsed = time.time() - opened_at_now
        if elapsed >= self.reset_timeout_seconds:
            pipe = self.client.pipeline(transaction=False)
            pipe.delete(self.key_opened_at)
            pipe.delete(self.key_failures)
            await pipe.execute()
