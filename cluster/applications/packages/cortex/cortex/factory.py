import abc
import collections.abc
import itertools
import typing

T = typing.TypeVar("T")


class LoadBalancingFactory(abc.ABC, typing.Generic[T]):
    @abc.abstractmethod
    async def construct(self, *args, **kwargs) -> T | None:
        raise NotImplementedError


class RoundRobinFactory(LoadBalancingFactory[T]):
    iterator: itertools.cycle | None

    def __init__(
        self,
        factories: collections.abc.Sequence[typing.Callable[[typing.Any], typing.Awaitable[T | None]]],
    ):
        self.iterator = itertools.cycle(factories) if factories else None

    async def construct(self, *args, **kwargs) -> T | None:
        if self.iterator is None:
            return None

        return await next(self.iterator)(*args, **kwargs)
