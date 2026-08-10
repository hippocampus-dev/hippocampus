import collections
import re
import typing

import slack_bolt.async_app
import slack_sdk.web.async_client

import cortex.brain
from . import RetryableAsyncWebClient


async def register(bolt: slack_bolt.async_app.AsyncApp, brain: cortex.brain.Brain):
    @bolt.action(re.compile(r"expand_\d+"))
    async def handle_expand(
        ack: slack_bolt.async_app.AsyncAck,
        client: slack_sdk.web.async_client.AsyncWebClient,
        body: collections.abc.Mapping[str, typing.Any],
    ):
        retryable_client = RetryableAsyncWebClient(client)

        value = body["actions"][0]["value"]
        b = await brain.restore(value)

        await retryable_client.chat_postEphemeral(
            channel=body["channel"]["id"],
            thread_ts=body["message"].get("thread_ts") or body["message"]["ts"],
            user=body["user"]["id"],
            text=value,
            blocks=[
                {
                    "type": "markdown",
                    "text": b.decode("utf-8"),
                },
            ],
        )

        await ack()
