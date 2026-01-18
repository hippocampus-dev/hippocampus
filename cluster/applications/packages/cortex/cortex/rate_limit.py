import abc
import time

import pydantic
import redis.asyncio
import redis.exceptions

import cortex.exceptions


class RateLimitInfo(pydantic.BaseModel):
    limit: int
    remaining: int
    reset_timestamp: int
    retry_after: int | None = None

    @property
    def is_exceeded(self) -> bool:
        return self.remaining <= 0


class RateLimiter(abc.ABC):
    @abc.abstractmethod
    async def take(self, key: str, amount: int):
        raise NotImplementedError

    @abc.abstractmethod
    async def remaining(self, key: str, limit: int) -> RateLimitInfo:
        raise NotImplementedError


class RedisFixedRateLimiter(RateLimiter):
    redis_client: redis.asyncio.Redis
    interval_seconds: int

    def __init__(self, redis_client: redis.asyncio.Redis, interval_seconds: int):
        self.redis_client = redis_client
        self.interval_seconds = interval_seconds

    async def remaining(self, key: str, limit: int) -> RateLimitInfo:
        try:
            current_value = await self.redis_client.get(self._redis_key(key))
            if current_value is None:
                return RateLimitInfo(
                    limit=limit,
                    remaining=limit,
                    reset_timestamp=int(time.time()) + self.interval_seconds
                )

            used_tokens = int(current_value)
            remaining_tokens = max(0, limit - used_tokens)

            ttl = await self.redis_client.ttl(self._redis_key(key))
            if ttl > 0:
                reset_timestamp = int(time.time()) + ttl
            else:
                reset_timestamp = int(time.time()) + self.interval_seconds

            retry_after_value = None
            if remaining_tokens <= 0:
                retry_after_value = reset_timestamp - int(time.time())
                retry_after_value = max(0, retry_after_value)

            return RateLimitInfo(
                limit=limit,
                remaining=remaining_tokens,
                reset_timestamp=reset_timestamp,
                retry_after=retry_after_value
            )
        except (redis.exceptions.ConnectionError, redis.exceptions.ReadOnlyError) as e:
            raise cortex.exceptions.RetryableError(e)

    async def take(self, key: str, amount: int):
        try:
            if await self.redis_client.get(self._redis_key(key)) is None:
                p = self.redis_client.pipeline()
                p.multi()
                p.incrby(self._redis_key(key), amount)
                p.expire(self._redis_key(key), self.interval_seconds)
                await p.execute()
            else:
                await self.redis_client.incrby(self._redis_key(key), amount)
        except (redis.exceptions.ConnectionError, redis.exceptions.ReadOnlyError) as e:
            raise cortex.exceptions.RetryableError(e)

    def _redis_key(self, key) -> str:
        return f"{key}:ratelimit:fixed"


class RedisSlidingRateLimiter(RateLimiter):
    redis_client: redis.asyncio.Redis
    interval_seconds: int

    def __init__(self, redis_client: redis.asyncio.Redis, interval_seconds: int):
        self.redis_client = redis_client
        self.interval_seconds = interval_seconds

    async def remaining(self, key: str, limit: int) -> RateLimitInfo:
        ts_ns = time.time()
        ts = int(ts_ns)
        window_seconds = ts - self.interval_seconds

        try:
            await self.redis_client.zremrangebyscore(self._redis_key(key), 0, window_seconds)
            results = await self.redis_client.zrangebyscore(self._redis_key(key), window_seconds, ts)
        except (redis.exceptions.ConnectionError, redis.exceptions.ReadOnlyError) as e:
            raise cortex.exceptions.RetryableError(e)

        if len(results) == 0:
            return RateLimitInfo(
                limit=limit,
                remaining=limit,
                reset_timestamp=ts + self.interval_seconds
            )

        used_tokens = sum([int(tokens.decode("utf-8").split(":")[0]) for tokens in results])
        remaining_tokens = max(0, limit - used_tokens)

        oldest_timestamp = min([int(tokens.decode("utf-8").split(":")[1]) for tokens in results])
        reset_timestamp = oldest_timestamp + self.interval_seconds

        retry_after_value = None
        if remaining_tokens <= 0:
            retry_after_value = reset_timestamp - int(time.time())
            retry_after_value = max(0, retry_after_value)

        return RateLimitInfo(
            limit=limit,
            remaining=remaining_tokens,
            reset_timestamp=reset_timestamp,
            retry_after=retry_after_value
        )

    async def take(self, key: str, amount: int):
        ts_ns = time.time()
        ts = int(ts_ns)

        try:
            if await self.redis_client.zcount(self._redis_key(key), "-inf", "+inf") == 0:
                p = self.redis_client.pipeline()
                p.multi()
                p.zadd(self._redis_key(key), {f"{amount}:{ts}": ts})
                p.expire(self._redis_key(key), self.interval_seconds)
                await p.execute()
            else:
                await self.redis_client.zadd(self._redis_key(key), {f"{amount}:{ts}": ts})
        except (redis.exceptions.ConnectionError, redis.exceptions.ReadOnlyError) as e:
            raise cortex.exceptions.RetryableError(e)

    def _redis_key(self, key) -> str:
        return f"{key}:ratelimit:sliding"
