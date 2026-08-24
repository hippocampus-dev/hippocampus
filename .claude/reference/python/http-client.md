# HTTP Client Pattern

How to use aiohttp and httpx in Python applications.

## Timeout Configuration

Always set explicit timeouts for HTTP clients:

```python
# aiohttp
timeout = aiohttp.ClientTimeout(total=30)
async with aiohttp.ClientSession(timeout=timeout) as session:
    async with session.post(url, json=data) as response:
        ...

# httpx
async with httpx.AsyncClient(timeout=30.0) as client:
    response = await client.post(url, json=data)
```

| Library | Default | Recommendation |
|---------|---------|----------------|
| aiohttp | 5 minutes (300s) | Set explicit timeout |
| httpx | 5 seconds | Set explicit timeout |

## Retry with Exponential Backoff

Use exponential backoff for transient failures:

```python
import asyncio

max_retries = 3
base_delay = 1.0

for attempt in range(max_retries):
    try:
        async with session.post(url, json=data) as response:
            if response.status >= 500 or response.status in (409, 429):
                if attempt < max_retries - 1:
                    await asyncio.sleep(base_delay * (2 ** attempt))
                    continue
            response.raise_for_status()
            return await response.json()
    except (aiohttp.ClientConnectionError, asyncio.TimeoutError):
        if attempt < max_retries - 1:
            await asyncio.sleep(base_delay * (2 ** attempt))
            continue
        raise
```

| Status Code | Action |
|-------------|--------|
| 409, 429 | Retry with backoff |
| 502, 503, 504 | Retry with backoff |
| 4xx (other) | Do not retry |
| Connection errors | Retry with backoff |

## Timestamp for Persistence

Use UTC for timestamps stored in databases or APIs:

```python
import datetime

# For persistence (documents, APIs)
now = datetime.datetime.now(datetime.timezone.utc).timestamp()

# For logging (local time acceptable)
now = datetime.datetime.now()
```

| Use Case | Pattern |
|----------|---------|
| Database timestamps | `datetime.datetime.now(datetime.timezone.utc).timestamp()` |
| API payloads | `datetime.datetime.now(datetime.timezone.utc).timestamp()` |
| Log messages | `datetime.datetime.now()` |
