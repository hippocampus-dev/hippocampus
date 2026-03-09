import asyncio
import time
import typing

import cortex.exceptions


class ThrottledFunction:
    func: typing.Callable[..., typing.Any]
    wait: int
    last_executed: float
    timeout: asyncio.Task | None
    throttled: bool

    def __init__(self, func: typing.Callable[..., typing.Any], wait: int):
        self.func = func
        self.wait = wait
        self.last_executed = 0
        self.timeout = None
        self.throttled = False

    @property
    def is_throttled(self):
        return self.throttled

    async def execute(self, *args):
        now = time.time()
        next_scheduled = self.last_executed + self.wait

        if self.timeout is not None or self.is_throttled:
            return

        async def wrapper():
            await asyncio.sleep(next_scheduled - now)
            self.timeout = None
            self.last_executed = now
            try:
                await self.func(*args)
            except cortex.exceptions.RetryableError:
                self.throttled = True

        self.timeout = asyncio.create_task(wrapper())

    async def immediately_execute(self, *args):
        await self.cancel()
        await self.func(*args)
        self.throttled = False

    async def cancel(self):
        if self.timeout is None:
            return
        self.timeout.cancel()
        try:
            await self.timeout
        except asyncio.CancelledError:
            pass
        self.timeout = None
