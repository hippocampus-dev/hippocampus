import collections.abc
import contextlib
import enum
import math
import time
import uuid

import redis.asyncio

_MAX_LATENCY_SAMPLES = 1000


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
    latency_rate_threshold: float
    latency_z_score_threshold: float
    latency_min_samples: int
    key_failures: str
    key_opened_at: str
    key_singleflight: str
    _key_prefix: str

    def __init__(
        self,
        client: redis.asyncio.Redis,
        name: str,
        failure_threshold: int = 3,
        reset_timeout_seconds: int = 60,
        evaluation_window_seconds: int = 120,
        latency_rate_threshold: float = 0.5,
        latency_z_score_threshold: float = 2.0,
        latency_min_samples: int = 10,
    ) -> None:
        self.client = client
        self.name = name
        self.failure_threshold = failure_threshold
        self.reset_timeout_seconds = reset_timeout_seconds
        self.evaluation_window_seconds = evaluation_window_seconds
        self.latency_rate_threshold = latency_rate_threshold
        self.latency_z_score_threshold = latency_z_score_threshold
        self.latency_min_samples = latency_min_samples
        self._key_prefix = f"circuit_breaker:{name}"
        self.key_opened_at = f"{self._key_prefix}:opened_at"
        self.key_failures = f"{self._key_prefix}:failures"
        self.key_singleflight = f"{self._key_prefix}:singleflight"

    def _latencies_key(self, segment: str | None) -> str:
        if segment is not None:
            return f"{self._key_prefix}:{segment}:latencies"
        return f"{self._key_prefix}:latencies"

    def _slow_calls_key(self, segment: str | None) -> str:
        if segment is not None:
            return f"{self._key_prefix}:{segment}:slow_calls"
        return f"{self._key_prefix}:slow_calls"

    @contextlib.asynccontextmanager
    async def call(
        self, segment: str | None = None
    ) -> collections.abc.AsyncGenerator[None, None]:
        state = await self._state()

        if state is State.OPEN:
            raise CircuitBreakerOpen()

        if state is State.HALF_OPEN:
            acquired = await self.client.set(
                self.key_singleflight, "1", nx=True, ex=self.reset_timeout_seconds
            )
            if not acquired:
                raise CircuitBreakerOpen()

        started_at = time.time()
        try:
            yield
        except CircuitBreakerFailure as e:
            await self._on_failure()
            raise e.__cause__ from None
        else:
            try:
                await self._on_success(time.time() - started_at, segment)
            except Exception:
                pass
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

    async def _on_success(
        self, latency_seconds: float, segment: str | None = None
    ) -> None:
        now = time.time()
        window_min = now - self.evaluation_window_seconds
        key_latencies = self._latencies_key(segment)
        key_slow_calls = self._slow_calls_key(segment)

        pipe = self.client.pipeline(transaction=False)
        pipe.zrangebyscore(key_latencies, window_min, "+inf")
        pipe.zremrangebyscore(key_slow_calls, "-inf", window_min)
        pipe.zcard(key_slow_calls)
        results = await pipe.execute()
        members = results[0]
        slow_count = results[2]

        latencies = _parse_latencies(members)
        is_slow = False
        if len(latencies) >= self.latency_min_samples:
            mean, stddev = _mean_stddev(latencies)
            if stddev > 0:
                z_score = (latency_seconds - mean) / stddev
                is_slow = z_score > self.latency_z_score_threshold

        pipe = self.client.pipeline(transaction=False)
        if is_slow:
            pipe.zadd(key_slow_calls, {uuid.uuid4().hex: now})
            pipe.expire(key_slow_calls, self.evaluation_window_seconds)
        else:
            member = f"{uuid.uuid4().hex}:{latency_seconds}"
            pipe.zadd(key_latencies, {member: now})
        pipe.zremrangebyscore(key_latencies, "-inf", window_min)
        pipe.zremrangebyrank(key_latencies, 0, -_MAX_LATENCY_SAMPLES - 1)
        pipe.expire(key_latencies, self.evaluation_window_seconds)
        await pipe.execute()

        slow_total = slow_count + (1 if is_slow else 0)
        total_count = len(latencies) + 1 + slow_count
        if slow_total / total_count >= self.latency_rate_threshold:
            await self.client.set(
                self.key_opened_at,
                str(now),
                ex=self.reset_timeout_seconds,
            )

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


def _parse_latencies(members: list[bytes]) -> list[float]:
    latencies = []
    for member in members:
        value = member.decode() if isinstance(member, bytes) else member
        parts = value.rsplit(":", 1)
        if len(parts) == 2:
            try:
                latencies.append(float(parts[1]))
            except ValueError:
                continue
    return latencies


def _mean_stddev(values: list[float]) -> tuple[float, float]:
    n = len(values)
    mean = sum(values) / n
    variance = sum(v * v for v in values) / n - mean * mean
    if variance < 0:
        variance = 0
    return mean, math.sqrt(variance)
