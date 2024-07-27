import abc
import time

import redis.asyncio
import redis.exceptions

import cortex.exceptions


class RateLimiter(abc.ABC):
    @abc.abstractmethod
    async def remaining(self, key: str) -> bool:
        raise NotImplementedError

    @abc.abstractmethod
    async def take(self, key: str, amount: int):
        raise NotImplementedError


class RedisFixedRateLimiter(RateLimiter):
    redis_client: redis.asyncio.Redis
    limit: int
    interval_seconds: int

    def __init__(self, redis_client: redis.asyncio.Redis, limit: int, interval_seconds: int):
        self.redis_client = redis_client
        self.limit = limit
        self.interval_seconds = interval_seconds

    async def remaining(self, key: str) -> bool:
        if (i := await self.redis_client.get(self._redis_key(key))) is None:
            return True

        return int(i) < self.limit

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
    limit: int
    interval_seconds: int

    def __init__(self, redis_client: redis.asyncio.Redis, limit: int, interval_seconds: int):
        self.redis_client = redis_client
        self.limit = limit
        self.interval_seconds = interval_seconds

    async def remaining(self, key: str) -> bool:
        ts_ns = time.time()
        ts = int(ts_ns)
        window_seconds = ts - self.interval_seconds

        try:
            await self.redis_client.zremrangebyscore(self._redis_key(key), 0, window_seconds)
            results = await self.redis_client.zrangebyscore(self._redis_key(key), window_seconds, ts)
        except (redis.exceptions.ConnectionError, redis.exceptions.ReadOnlyError) as e:
            raise cortex.exceptions.RetryableError(e)

        if len(results) == 0:
            return True

        return sum([int(tokens.decode("utf-8").split(":")[0]) for tokens in results]) < self.limit

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
