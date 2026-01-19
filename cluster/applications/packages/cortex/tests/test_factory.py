import collections.abc
import typing
import unittest

import cortex.factory


def factory_generator(n: int) -> typing.Callable[[int], typing.Awaitable[int | None]]:
    async def factory(m: int):
        return n * m

    return factory


def string_factory_generator(n: int) -> typing.Callable[[str], typing.Awaitable[int | None]]:
    async def factory(key: str):
        return n

    return factory


class TestCase(unittest.IsolatedAsyncioTestCase):
    async def test_round_robin(self):
        round_robin_factory = cortex.factory.RoundRobinFactory([
            factory_generator(i) for i in range(1, 4)
        ])

        self.assertEqual(await round_robin_factory.construct(1), 1)
        self.assertEqual(await round_robin_factory.construct(2), 4)
        self.assertEqual(await round_robin_factory.construct(3), 9)
        self.assertEqual(await round_robin_factory.construct(4), 4)

    async def test_consistent_hash_same_key_same_node(self):
        consistent_hash_factory = cortex.factory.ConsistentHashFactory([
            string_factory_generator(i) for i in range(3)
        ])

        result1 = await consistent_hash_factory.construct("channel_1")
        result2 = await consistent_hash_factory.construct("channel_1")
        result3 = await consistent_hash_factory.construct("channel_1")

        self.assertEqual(result1, result2)
        self.assertEqual(result2, result3)

    async def test_consistent_hash_distribution(self):
        consistent_hash_factory = cortex.factory.ConsistentHashFactory([
            string_factory_generator(i) for i in range(3)
        ])

        results: collections.abc.MutableMapping[int, int] = {}
        for i in range(100):
            result = await consistent_hash_factory.construct(f"channel_{i}")
            results[result] = results.get(result, 0) + 1

        self.assertEqual(len(results), 3)
        for count in results.values():
            self.assertGreater(count, 10)

    async def test_consistent_hash_empty(self):
        consistent_hash_factory = cortex.factory.ConsistentHashFactory([])

        result = await consistent_hash_factory.construct("channel_1")
        self.assertIsNone(result)

    async def test_consistent_hash_ring_add_remove(self):
        ring = cortex.factory.ConsistentHashRing(3)

        node1 = ring.get_node("key1")
        node2 = ring.get_node("key2")

        ring.add_node(3)

        node1_after_add = ring.get_node("key1")
        node2_after_add = ring.get_node("key2")

        self.assertTrue(node1 == node1_after_add or node1_after_add == 3)
        self.assertTrue(node2 == node2_after_add or node2_after_add == 3)


if __name__ == "__main__":
    unittest.main()
