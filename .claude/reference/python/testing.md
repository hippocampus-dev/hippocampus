# Python Testing Patterns

How to write consistent tests in Python packages.

## Framework Choice

Use `unittest` from the standard library.
Do not use `pytest`.

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

Swap both lines when nothing in the file awaits:

```python
class TestCase(unittest.TestCase):
    def test_round_robin(self):
        # test logic
        self.assertEqual(...)
```

| Practice | Reason |
|----------|--------|
| Class name literally `TestCase` | One class per file; suffix is redundant |
| `if __name__ == "__main__": unittest.main()` footer | Allow running `python test_{module}.py` directly |
| Full module paths in imports (`import cortex.factory`) | Matches `python.md` convention |

## Calling a FastAPI Handler Directly

Awaiting a handler as a plain coroutine drives a WebSocket or streaming endpoint without a server, but FastAPI resolves none of its parameters on that path.

| Declared parameter | Through FastAPI | Awaited directly |
|--------------------|-----------------|------------------|
| `fastapi.Header(default="anonymous")` | `"anonymous"` | a `fastapi.params.Header` |
| `fastapi.Query(default="a")` | `"a"` | a `fastapi.params.Query` |

Pass every such parameter at the call site, as `open_connection` in `cluster/applications/api/tests/test_main.py` does.
The object itself raises nothing, so it surfaces wherever the handler first treats it as the value - serializing it, or using it as a dictionary key.
