import argparse
import asyncio
import sys

import pydantic
import slack_sdk.web.async_client
import slack_sdk.web.async_slack_response


class Settings(pydantic.BaseSettings):
    slack_bot_token: str
    slack_signing_secret: str
    slack_bot_member_id: str

    class Config:
        env_file = ".env"


s = Settings()


async def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--channel", required=True)
    parser.add_argument("--messages", required=True, nargs="+")
    args = parser.parse_args()

    channel = args.channel
    client = slack_sdk.web.async_client.AsyncWebClient(token=s.slack_bot_token)

    async def post_message_returns_permalink(message: str) -> str:
        response = await client.chat_postMessage(channel=channel, text=message)
        if response["ok"]:
            response = await client.chat_getPermalink(channel=channel, message_ts=response["ts"])
            if response["ok"]:
                return response["permalink"]

        print(response["error"], file=sys.stderr)

    links = await asyncio.gather(*[post_message_returns_permalink(message) for message in args.messages])

    print("\n".join(links))


if __name__ == "__main__":
    asyncio.run(main())
