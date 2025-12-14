import asyncio
import collections.abc
import dataclasses
import functools
import random
import re
import typing
import urllib.parse

import mistune.renderers.markdown
import slack_sdk.errors
import slack_sdk.web.async_client
import slack_sdk.web.async_slack_response

import cortex.exceptions
import cortex.llm.openai.model
from .i18n import Locale, translate


class SlackContext():
    user: collections.abc.Mapping[str, typing.Any]
    _locale: Locale | None

    def __init__(
        self,
        user: collections.abc.Mapping[str, typing.Any],
    ):
        self.user = user
        self._locale = None

    @property
    def limit(self) -> int | None:
        return None

    @property
    def locale(self) -> Locale:
        if self._locale is not None:
            return self._locale

        match self.user.get("profile", {}).get("team"):
            case "T02BY6T3YE6":
                return Locale.Japanese
            case _:
                return Locale.English

    @locale.setter
    def locale(self, value: Locale):
        self._locale = value


class SlackMarkdownRenderer(mistune.renderers.markdown.MarkdownRenderer):
    url_shortener: cortex.URLShortener

    def __init__(self, url_shortener: cortex.URLShortener):
        super().__init__()
        self.url_shortener = url_shortener

    def text(self, token: collections.abc.Mapping[str, typing.Any], state: mistune.BlockState) -> str:
        token = token["raw"]
        url = urllib.parse.urlparse(token)

        if url.scheme in ["http", "https"] and len(token) > 1024:
            token = self.url_shortener.shorten(token)

        return token

    def emphasis(self, token: collections.abc.Mapping[str, typing.Any], state: mistune.BlockState) -> str:
        return f"_{self.render_children(token, state)}_"

    def strong(self, token: collections.abc.Mapping[str, typing.Any], state: mistune.BlockState) -> str:
        return f"*{self.render_children(token, state)}*"

    def image(self, token: collections.abc.Mapping[str, typing.Any], state: mistune.BlockState) -> str:
        return self.link(token, state)

    def link(self, token: collections.abc.Mapping[str, typing.Any], state: mistune.BlockState) -> str:
        url = token["attrs"]["url"]

        if len(url) > 1024:
            url = self.url_shortener.shorten(url)

        return f"<{url}|{self.render_children(token, state)}>"

    def block_code(self, token: collections.abc.Mapping[str, typing.Any], state: mistune.BlockState) -> str:
        return f"```{token['raw']}```\n"


PATTERNS: collections.abc.Mapping[str, tuple[str, str]] = {}
COMPILED_PATTERNS = {pattern_name: re.compile(pattern) for pattern_name, (pattern, _) in PATTERNS.items()}


def transform_message(message: str, renderer: mistune.renderers.markdown.MarkdownRenderer) -> str:
    transformed_message = mistune.Markdown(renderer=renderer)(message)

    for pattern_name, compiled_pattern in COMPILED_PATTERNS.items():
        transformed_message = compiled_pattern.sub(PATTERNS[pattern_name][1], transformed_message)

    return transformed_message


def retry(attempts: int, next_delay: int, max_delay: int | None = None):
    def decorator(f):
        @functools.wraps(f)
        async def wrapper(*args, **kwargs):
            i = 0

            async def _retry(_attempts: int, _next_delay: int, _max_delay: int | None = None):
                nonlocal i

                if _max_delay is None:
                    delay = _next_delay
                else:
                    delay = min(_next_delay, _max_delay)

                try:
                    return await f(*args, **kwargs)
                except asyncio.TimeoutError as e:
                    if i >= _attempts:
                        raise cortex.exceptions.RetryableError(e) from e

                    await asyncio.sleep(delay * random.random())

                    if _next_delay <= (2 ** 63 - 1) / 2:
                        _next_delay *= 2
                    else:
                        _next_delay = 2 ** 63 - 1

                    i += 1
                    return await _retry(_next_delay, _max_delay)
                except slack_sdk.errors.SlackApiError as e:
                    if "error" in e.response:
                        # Ignore errors
                        if e.response["error"] in [
                            "channel_not_found",
                            # chat.postMessage https://api.slack.com/methods/chat.postMessage#errors
                            "not_in_channel",
                            # chat.postEphemeral https://api.slack.com/methods/chat.postEphemeral#errors
                            "user_not_in_channel",
                            # chat.delete https://api.slack.com/methods/chat.delete#errors
                            "message_not_found",
                            # reactions.add https://api.slack.com/methods/reactions.add#errors
                            "already_reacted",
                        ]:
                            return

                        # Raise errors that are not retryable
                        if e.response["error"] not in ["ratelimited", "fatal_error"]:
                            raise e

                    if i >= _attempts:
                        raise cortex.exceptions.RetryableError(e) from e

                    if e.response.status_code == 429 and "Retry-After" in e.response.headers:
                        delay = int(e.response.headers["Retry-After"])

                    await asyncio.sleep(delay * random.random())

                    if _next_delay <= (2 ** 63 - 1) / 2:
                        _next_delay *= 2
                    else:
                        _next_delay = 2 ** 63 - 1

                    i += 1
                    return await _retry(_attempts, _next_delay, _max_delay)

            return await _retry(attempts, next_delay, max_delay)

        return wrapper

    return decorator


@dataclasses.dataclass
class RetryableAsyncWebClient(slack_sdk.web.async_client.AsyncWebClient):
    client: slack_sdk.web.async_client.AsyncWebClient

    def __getattribute__(self, name):
        if name in ["client"]:
            return object.__getattribute__(self, name)

        attr = getattr(self.client, name)

        if asyncio.iscoroutinefunction(attr):
            @retry(attempts=3, next_delay=1, max_delay=10)
            async def a(*args, **kwargs):
                return await attr(*args, **kwargs)

            return a

        if callable(attr):
            @retry(attempts=3, next_delay=1, max_delay=10)
            def f(*args, **kwargs):
                return attr(*args, **kwargs)

            return f

        return attr
