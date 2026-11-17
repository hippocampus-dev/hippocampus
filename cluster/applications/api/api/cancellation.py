import asyncio
import collections.abc
import contextlib

import fastapi


@contextlib.asynccontextmanager
async def cancel_on_disconnect(
    request: fastapi.Request,
) -> collections.abc.AsyncIterator[None]:
    current = asyncio.current_task()
    if current is None:
        yield
        return

    finished = False

    async def watch() -> None:
        try:
            while True:
                message = await request._receive()
                if message.get("type") == "http.disconnect":
                    if not finished:
                        current.cancel()
                    return
        except asyncio.CancelledError:
            pass

    watcher = asyncio.create_task(watch())
    try:
        yield
    finally:
        finished = True
        if not watcher.done():
            watcher.cancel()
        with contextlib.suppress(asyncio.CancelledError):
            await watcher
