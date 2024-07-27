import asyncio
import collections.abc
import dataclasses
import functools
import random
import re
import typing

import mistune.renderers.markdown
import slack_sdk.errors
import slack_sdk.web.async_client
import slack_sdk.web.async_slack_response
import tiktoken

import cortex.exceptions
import cortex.llm.openai.agent
import cortex.llm.openai.model
from .i18n import Locale, translate

CHAT_UPDATE_MAX_BYTES = 4000


class SlackContext(cortex.llm.openai.agent.Context):
    original_client: slack_sdk.web.async_client.AsyncWebClient
    client: slack_sdk.web.async_client.AsyncWebClient
    event: collections.abc.Mapping[str, typing.Any]
    channel: collections.abc.Mapping[str, typing.Any]
    user: collections.abc.Mapping[str, typing.Any]
    progress: slack_sdk.web.async_slack_response.AsyncSlackResponse | None
    progress_function_stack: collections.abc.MutableSequence[str]
    _locale: Locale | None

    def __init__(
        self,
        context_id: str,
        memory_type: cortex.llm.openai.agent.MemoryType,
        embedding_model: cortex.llm.openai.model.EmbeddingModel,
        encoder: tiktoken.Encoding,
        original_client: slack_sdk.web.async_client.AsyncWebClient,
        client: slack_sdk.web.async_client.AsyncWebClient,
        event: collections.abc.Mapping[str, typing.Any],
        channel: collections.abc.Mapping[str, typing.Any],
        user: collections.abc.Mapping[str, typing.Any],
        progress: slack_sdk.web.async_slack_response.AsyncSlackResponse | None,
    ):
        super().__init__(context_id, memory_type, embedding_model, encoder)
        self.original_client = original_client
        self.client = client
        self.event = event
        self.channel = channel
        self.user = user
        self.progress = progress
        self.progress_function_stack = []
        self._locale = None

    @property
    def capability(self) -> cortex.llm.openai.agent.Capability:
        if self.user.get("is_restricted") or self.user.get("is_ultra_restricted"):
            return cortex.llm.openai.agent.Capability.DEFAULT

        match self.user.get("profile", {}).get("team"):
            case "T02BY6T3YE6":
                return cortex.llm.openai.agent.Capability.ALL
            case _:
                return cortex.llm.openai.agent.Capability.DEFAULT

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

    async def report_progress(self, message: str, stage: cortex.llm.openai.agent.ProgressStage):
        if self.progress is not None:
            match stage:
                case (cortex.llm.openai.agent.ProgressStage.FunctionCalling):
                    self.progress_function_stack.append(message)
                    tasks = "\n---\n".join(self.progress_function_stack)

                    header = translate("The following task is being executed:", self.locale)
                    await self.client.chat_update(
                        channel=self.progress["channel"],
                        ts=self.progress["ts"],
                        text=f"{header}"
                             f"```"
                             f"{tasks}"
                             f"```",
                    )
                case (cortex.llm.openai.agent.ProgressStage.ResponseStarting):
                    self.progress_function_stack.clear()


PATTERNS: collections.abc.Mapping[str, tuple[str, str]] = {
    "hyperlinks": (r'\[([^\]]+)\]\((https?://[^\s:{}|\^\[\]`]+)\)', r'<\2|\1>'),
}
COMPILED_PATTERNS = {pattern_name: re.compile(pattern) for pattern_name, (pattern, _) in PATTERNS.items()}


class SlackMarkdownRenderer(mistune.renderers.markdown.MarkdownRenderer):
    def emphasis(self, token: collections.abc.Mapping[str, typing.Any], state: mistune.BlockState) -> str:
        return f"_{self.render_children(token, state)}_"

    def strong(self, token: collections.abc.Mapping[str, typing.Any], state: mistune.BlockState) -> str:
        return f"*{self.render_children(token, state)}*"


def transform_message(message: str) -> str:
    transformed_message = mistune.Markdown(renderer=SlackMarkdownRenderer())(message)

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
