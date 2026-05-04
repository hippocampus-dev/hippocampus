# Python Testing Patterns

How to write consistent tests in Python packages.

## Framework Choice

Use `unittest` from the standard library. Do not use `pytest`.

| Base Class | When |
|------------|------|
| `unittest.TestCase` | Synchronous code |
| `unittest.IsolatedAsyncioTestCase` | `async def` code under test |

## File Layout

| Path | Purpose |
|------|---------|
| `{package}/tests/test_{module}.py` | One test module per source module |

## Test Module Structure

```python
import unittest

import cortex.factory


class TestCase(unittest.IsolatedAsyncioTestCase):
    async def test_round_robin(self):
        # test logic
        self.assertEqual(...)


if __name__ == "__main__":
    unittest.main()
```

| Practice | Reason |
|----------|--------|
| Class name literally `TestCase` | One class per file; suffix is redundant |
| `if __name__ == "__main__": unittest.main()` footer | Allow running `python test_{module}.py` directly |
| Full module paths in imports (`import cortex.factory`) | Matches `python.md` convention |
