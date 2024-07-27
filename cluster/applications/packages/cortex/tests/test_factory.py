import typing
import unittest

import cortex.factory


def factory_generator(n: int) -> typing.Callable[[int], typing.Awaitable[int | None]]:
    async def factory(m: int):
        return n * m

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


if __name__ == "__main__":
    unittest.main()
