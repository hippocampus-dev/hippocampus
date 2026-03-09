import collections.abc
import json
import typing

import pydantic
import slack_bolt.async_app
import slack_bolt.context.ack.async_ack
import slack_sdk.web.async_client

import bot.slack
import cortex.brain


class PrivateMetadata(pydantic.BaseModel):
    channel: str
    ts: str


REPORT_CALLBACK_ID = "report"


async def register(bolt: slack_bolt.async_app.AsyncApp, brain: cortex.brain.Brain):
    @bolt.action(REPORT_CALLBACK_ID)
    async def handle_report(
        ack: slack_bolt.context.ack.async_ack.AsyncAck,
        client: slack_sdk.web.async_client.AsyncWebClient,
        body: collections.abc.Mapping[str, typing.Any],
    ):
        await ack()

        retryable_client = bot.slack.RetryableAsyncWebClient(client)

        private_metadata = PrivateMetadata.model_validate_json(
            body["actions"][0]["value"]
        )

        replies = await retryable_client.conversations_replies(
            channel=private_metadata.channel,
            ts=private_metadata.ts,
        )

        await brain.save(
            f"reports/{private_metadata.channel}-{private_metadata.ts}",
            json.dumps(replies["messages"]).encode("utf-8"),
        )


def build_reporting_block(
    context: bot.slack.SlackContext,
) -> collections.abc.Sequence[dict[str, typing.Any]]:
    private_metadata = PrivateMetadata(
        channel=context.event["channel"],
        ts=context.event.get("thread_ts") or context.event["ts"],
    )
    return {
        "type": "section",
        "text": {
            "type": "mrkdwn",
            "text": bot.slack.i18n.translate(
                "Click `Report' to send conversations in this thread to the development team.",
                locale=context.locale,
            ),
        },
        "accessory": {
            "type": "button",
            "style": "danger",
            "text": {
                "type": "plain_text",
                "text": bot.slack.i18n.translate(
                    "Report",
                    locale=context.locale,
                ),
            },
            "confirm": {
                "title": {
                    "type": "plain_text",
                    "text": bot.slack.i18n.translate(
                        "Do you want to report it?",
                        locale=context.locale,
                    ),
                },
                "text": {
                    "type": "mrkdwn",
                    "text": bot.slack.i18n.translate(
                        "Are you sure you want this thread conversations sent?",
                        locale=context.locale,
                    ),
                },
                "deny": {
                    "type": "plain_text",
                    "text": bot.slack.i18n.translate(
                        "Cancel",
                        locale=context.locale,
                    ),
                },
                "confirm": {
                    "type": "plain_text",
                    "text": bot.slack.i18n.translate(
                        "Report",
                        locale=context.locale,
                    ),
                },
            },
            "action_id": REPORT_CALLBACK_ID,
            "value": json.dumps(private_metadata.model_dump(), ensure_ascii=False),
        },
    }
