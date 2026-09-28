import argparse
import asyncio
import collections.abc
import hashlib
import hmac
import json
import pprint
import re
import time

import aiohttp
import pydantic
import pydantic_settings
import slack_sdk.web.async_client


class Element(pydantic.BaseModel):
    type: str
    user_id: str | None = None
    text: str | None = None


class BlockElement(pydantic.BaseModel):
    type: str
    elements: collections.abc.Sequence[Element]


class Block(pydantic.BaseModel):
    type: str
    block_id: str
    elements: collections.abc.Sequence[BlockElement]


class Event(pydantic.BaseModel):
    type: str
    subtype: str | None = None
    text: str
    user: str | None = None
    username: str | None = None
    bot_id: str | None = None
    ts: str
    blocks: collections.abc.Sequence[Block]
    team: str
    thread_ts: str | None
    channel: str
    event_ts: str
    channel_type: str | None


class SlackEvent(pydantic.BaseModel):
    event: Event
    type: str
    is_ext_shared_channel: bool


class Settings(pydantic_settings.BaseSettings):
    model_config = pydantic_settings.SettingsConfigDict(extra="allow", env_file=".env")

    host: str = "127.0.0.1"
    port: int = 8080

    slack_bot_token: str
    slack_signing_secret: str
    slack_bot_member_id: str


s = Settings()


def signature(key: bytes, msg: bytes) -> str:
    return f"v0={hmac.new(key, msg, hashlib.sha256).hexdigest()}"


async def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--url", required=True)
    args = parser.parse_args()

    pattern = r"https://\S+.slack.com/archives/(\S+)/p(\d+)"

    if (m := re.match(pattern, args.url)) is None:
        raise ValueError("Invalid URL")

    channel = m.group(1)
    ts = m.group(2)[:-6] + "." + m.group(2)[-6:]
    response = await slack_sdk.web.async_client.AsyncWebClient(
        token=s.slack_bot_token
    ).conversations_replies(
        channel=channel,
        ts=ts,
        limit=1,
    )
    message = response["messages"][0]

    async with aiohttp.ClientSession() as session:
        ts_ns = time.time()
        ts = int(ts_ns)
        request = SlackEvent(
            event=Event(
                type="app_mention"
                if f"<@{s.slack_bot_member_id}>" in message["text"]
                else "message",
                subtype=message.get("subtype"),
                text=message["text"],
                user=message.get("user"),
                username=message.get("username"),
                bot_id=message.get("bot_id"),
                ts=message["ts"],
                blocks=message["blocks"],
                team=message.get("team", message.get("user_team", "")),
                channel=channel,
                thread_ts=message.get("thread_ts"),
                event_ts=str(ts_ns),
                channel_type="im" if channel.startswith("D") else None,
            ),
            type="event_callback",
            is_ext_shared_channel=False,
        )
        body = json.dumps(request.model_dump(exclude_none=True), ensure_ascii=False)
        async with session.post(
            f"http://{s.host}:{s.port}/slack/events",
            headers={
                "X-Slack-Request-Timestamp": str(ts),
                "X-Slack-Signature": signature(
                    s.slack_signing_secret.encode("utf-8"),
                    f"v0:{str(ts)}:{body}".encode("utf-8"),
                ),
            },
            data=body,
        ) as response:
            response.raise_for_status()

        pprint.pprint(request.model_dump())


if __name__ == "__main__":
    asyncio.run(main())
