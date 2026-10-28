import argparse
import asyncio
import re

import pydantic_settings
import slack_sdk.web.async_client


class Settings(pydantic_settings.BaseSettings):
    model_config = pydantic_settings.SettingsConfigDict(extra="allow", env_file=".env")

    slack_bot_token: str
    slack_signing_secret: str
    slack_bot_member_id: str


s = Settings()


async def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--url", required=True)
    args = parser.parse_args()

    pattern = r"https://\S+.slack.com/archives/(\S+)/p(\d+)"
    if (m := re.match(pattern, args.url)) is None:
        raise ValueError("Invalid URL")

    channel = m.group(1)
    ts = m.group(2)[:-6] + "." + m.group(2)[-6:]
    client = slack_sdk.web.async_client.AsyncWebClient(token=s.slack_bot_token)
    response = await client.conversations_replies(
        channel=channel,
        ts=ts,
    )
    message = response["messages"][0]

    await asyncio.gather(
        *[
            client.chat_update(
                channel=channel,
                ts=message["ts"],
                text=str(i),
            )
            for i in range(60)
        ]
    )


if __name__ == "__main__":
    asyncio.run(main())
