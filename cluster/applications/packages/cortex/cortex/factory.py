import abc
import bisect
import collections.abc
import hashlib
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
        factories: collections.abc.Sequence[typing.Callable[..., typing.Awaitable[T | None]]],
    ):
        self.iterator = itertools.cycle(factories) if factories else None

    async def construct(self, *args, **kwargs) -> T | None:
        if self.iterator is None:
            return None

        return await next(self.iterator)(*args, **kwargs)


class ConsistentHashRing:
    virtual_nodes_count: int
    ring: list[tuple[int, int]]
    existing_nodes: set[int]

    def __init__(self, node_count: int, virtual_nodes_count: int = 150):
        self.virtual_nodes_count = virtual_nodes_count
        self.ring = []
        self.existing_nodes = set()

        for node_index in range(node_count):
            for virtual_index in range(virtual_nodes_count):
                key = f"{node_index}:{virtual_index}"
                hash_value = self._hash(key)
                self.ring.append((hash_value, node_index))

            self.existing_nodes.add(node_index)
        self.ring.sort(key=lambda x: x[0])

    def add_node(self, node_index: int) -> None:
        if node_index < 0:
            raise ValueError(f"Node index must be non-negative, got {node_index}")

        if node_index in self.existing_nodes:
            raise ValueError(f"Node {node_index} already exists in the ring")

        for virtual_index in range(self.virtual_nodes_count):
            key = f"{node_index}:{virtual_index}"
            hash_value = self._hash(key)
            self.ring.append((hash_value, node_index))

        self.existing_nodes.add(node_index)
        self.ring.sort(key=lambda x: x[0])

    def remove_node(self, node_index: int) -> None:
        if not self.ring:
            raise ValueError("No nodes available in the ring")

        if node_index not in self.existing_nodes:
            raise ValueError(f"Node {node_index} does not exist in the ring")

        self.ring = [(hash_value, node) for hash_value, node in self.ring if node != node_index]
        self.existing_nodes.discard(node_index)

    def get_node(self, key: str) -> int:
        if not self.ring:
            raise ValueError("No nodes available in the ring")

        hash_value = self._hash(key)
        index = bisect.bisect_right(self.ring, hash_value, key=lambda x: x[0])
        return self.ring[index % len(self.ring)][1]

    def _hash(self, key: str) -> int:
        return int.from_bytes(
            hashlib.blake2s(key.encode("utf-8"), digest_size=8).digest(),
            byteorder="big"
        )


class ConsistentHashFactory(LoadBalancingFactory[T]):
    factories: collections.abc.Sequence[typing.Callable[..., typing.Awaitable[T | None]]]
    ring: ConsistentHashRing | None

    def __init__(
        self,
        factories: collections.abc.Sequence[typing.Callable[..., typing.Awaitable[T | None]]],
        virtual_nodes_count: int = 150,
    ):
        self.factories = factories
        self.ring = ConsistentHashRing(len(factories), virtual_nodes_count) if factories else None

    async def construct(self, key: str, *args, **kwargs) -> T | None:
        if self.ring is None or not self.factories or not self.ring.ring:
            return None

        node_index = self.ring.get_node(key)
        factory = self.factories[node_index]
        return await factory(key, *args, **kwargs)


def jump_consistent_hash(key: str, num_buckets: int) -> int:
    key_hash = int.from_bytes(
        hashlib.blake2s(key.encode("utf-8"), digest_size=8).digest(),
        byteorder="big",
    )

    b = -1
    j = 0
    while j < num_buckets:
        b = j
        key_hash = ((key_hash * 2862933555777941757) + 1) & 0xFFFFFFFFFFFFFFFF
        j = int((b + 1) * ((1 << 31) / ((key_hash >> 33) + 1)))

    return b


class JumpConsistentHashFactory(LoadBalancingFactory[T]):
    factories: collections.abc.Sequence[typing.Callable[..., typing.Awaitable[T | None]]]

    def __init__(
        self,
        factories: collections.abc.Sequence[typing.Callable[..., typing.Awaitable[T | None]]],
    ):
        self.factories = factories

    async def construct(self, key: str, *args, **kwargs) -> T | None:
        if not self.factories:
            return None

        node_index = jump_consistent_hash(key, len(self.factories))
        factory = self.factories[node_index]
        return await factory(key, *args, **kwargs)
