import asyncio
import unittest

import cortex.exceptions
import cortex.throttled_function


class TestCase(unittest.IsolatedAsyncioTestCase):
    async def test_cancel(self):
        i = 0

        async def func() -> int:
            nonlocal i

            i += 1
            if i % 2 == 0:
                raise cortex.exceptions.RetryableError

            await asyncio.sleep(0.1)
            return i

        throttled_function = cortex.throttled_function.ThrottledFunction(func, 0)

        await throttled_function.execute()
        await throttled_function.cancel()

        await asyncio.sleep(0.1)

        self.assertEqual(i, 0)
        self.assertFalse(throttled_function.is_throttled)

    async def test_throttling(self):
        i = 0

        async def func() -> int:
            nonlocal i

            i += 1
            if i % 2 == 0:
                raise cortex.exceptions.RetryableError

            await asyncio.sleep(0.1)
            return i

        throttled_function = cortex.throttled_function.ThrottledFunction(func, 0)

        await throttled_function.execute()
        await throttled_function.execute()

        await asyncio.sleep(0.1)

        self.assertEqual(i, 1)
        self.assertFalse(throttled_function.is_throttled)

    async def test_throttled(self):
        i = 0

        async def func() -> int:
            nonlocal i

            i += 1
            if i % 2 == 0:
                raise cortex.exceptions.RetryableError

            await asyncio.sleep(0.1)
            return i

        throttled_function = cortex.throttled_function.ThrottledFunction(func, 0)

        await throttled_function.execute()

        await asyncio.sleep(0.1)

        self.assertEqual(i, 1)
        self.assertFalse(throttled_function.is_throttled)

        await throttled_function.execute()

        await asyncio.sleep(0.1)

        self.assertEqual(i, 2)
        self.assertTrue(throttled_function.is_throttled)

    async def test_immediately_execute(self):
        i = 0

        async def func() -> int:
            nonlocal i

            i += 1
            if i % 2 == 0:
                raise cortex.exceptions.RetryableError

            await asyncio.sleep(0.1)
            return i

        throttled_function = cortex.throttled_function.ThrottledFunction(func, 0)

        await throttled_function.execute()
        await throttled_function.immediately_execute()

        await asyncio.sleep(0.1)

        self.assertEqual(i, 1)
        self.assertFalse(throttled_function.is_throttled)

    async def test_immediately_execute_throttled(self):
        i = 0

        async def func() -> int:
            nonlocal i

            i += 1
            if i % 2 == 0:
                raise cortex.exceptions.RetryableError

            await asyncio.sleep(0.1)
            return i

        throttled_function = cortex.throttled_function.ThrottledFunction(func, 0)

        await throttled_function.execute()

        await asyncio.sleep(0.1)

        self.assertEqual(i, 1)
        self.assertFalse(throttled_function.is_throttled)

        await throttled_function.execute()

        await asyncio.sleep(0.1)

        self.assertEqual(i, 2)
        self.assertTrue(throttled_function.is_throttled)

        await throttled_function.immediately_execute()

        await asyncio.sleep(0.1)

        self.assertEqual(i, 3)
        self.assertFalse(throttled_function.is_throttled)


if __name__ == "__main__":
    unittest.main()
