import asyncio
import collections.abc
import enum
import json
import typing

import aiohttp
import pydantic
import slack_bolt.async_app
import slack_sdk
import slack_sdk.web.async_client

import bot.slack.i18n
import cortex.brain


class Toggle(enum.StrEnum):
    On = "on"
    Off = "off"


class Avatar(pydantic.BaseModel):
    name: str
    icon_url: str
    instruction: str
    locale: bot.slack.i18n.Locale | None
    enabled: Toggle = Toggle.On


class Avatars(pydantic.BaseModel, collections.abc.MutableMapping):
    __root__: typing.Dict[str, Avatar]

    def __len__(self) -> int:
        return self.__root__.__len__()

    def __iter__(self) -> typing.Iterator[str]:
        return self.__root__.__iter__()

    def __getitem__(self, __key: str) -> Avatar:
        return self.__root__.__getitem__(__key)

    def __setitem__(self, __key: str, __value: Avatar) -> None:
        return self.__root__.__setitem__(__key, __value)

    def __delitem__(self, __key: str) -> None:
        return self.__root__.__delitem__(__key)


class PrivateMetadata(pydantic.BaseModel):
    channel_id: str
    user_id: str


NAME_BLOCK_ID = "name"
ICON_URL_BLOCK_ID = "icon_url"
INSTRUCTION_BLOCK_ID = "instruction"
LOCALE_BLOCK_ID = "locale"
TOGGLE_BLOCK_ID = "settings"

NAME_ACTION_ID = "name"
ICON_URL_ACTION_ID = "icon_url"
INSTRUCTION_ACTION_ID = "instruction"
LOCALE_ACTION_ID = "locale"
TOGGLE_ACTION_ID = "settings"

EDIT_AVATAR_CALLBACK_ID = "edit_avatar"

OPTION_VALUE_LIMIT = 20


async def register(bolt: slack_bolt.async_app.AsyncApp, brain: cortex.brain.Brain):
    @bolt.view(EDIT_AVATAR_CALLBACK_ID)
    async def handle_edit_avatar(
        ack: slack_bolt.async_app.AsyncAck,
        client: slack_sdk.web.async_client.AsyncWebClient,
        body: collections.abc.Mapping[str, typing.Any],
    ):
        retryable_client = bot.slack.RetryableAsyncWebClient(client)

        private_metadata = PrivateMetadata.parse_raw(body["view"]["private_metadata"])

        values = body["view"]["state"]["values"]
        avatar = Avatar(
            name=values[NAME_BLOCK_ID][NAME_ACTION_ID]["value"],
            icon_url=values[ICON_URL_BLOCK_ID][ICON_URL_ACTION_ID]["value"],
            instruction=values[INSTRUCTION_BLOCK_ID][INSTRUCTION_ACTION_ID]["value"],
            locale=values[LOCALE_BLOCK_ID][LOCALE_ACTION_ID]["selected_option"]["value"],
            enabled=values[TOGGLE_BLOCK_ID][TOGGLE_ACTION_ID]["selected_option"]["value"],
        )

        errors = {}
        if len(avatar.name) > OPTION_VALUE_LIMIT:
            errors[NAME_BLOCK_ID] = f"タイトルは{OPTION_VALUE_LIMIT}文字以下にしてください。"

        try:
            async with aiohttp.ClientSession(trust_env=True) as session:
                async with session.get(avatar.icon_url) as response:
                    if response.status != 200:
                        errors[ICON_URL_BLOCK_ID] = "アイコン画像のURLにアクセスできませんでした。"
        except aiohttp.InvalidURL:
            errors[ICON_URL_BLOCK_ID] = "アイコン画像のURLが無効です。"
        except (
            aiohttp.ClientConnectionError,  # ECONNREFUSED, EPIPE, ECONNRESET
            aiohttp.client_exceptions.ServerDisconnectedError,
            asyncio.TimeoutError,
        ):
            errors[ICON_URL_BLOCK_ID] = "アイコン画像のURLにアクセスできなかったため、再度お試しください。"

        if errors:
            await ack(response_action="errors", errors=errors)
            return

        restored_avatars = await brain.restore("avatars")
        avatars = (
            Avatars.parse_raw(restored_avatars)
            if restored_avatars is not None
            else Avatars.parse_obj({})
        )

        avatars[private_metadata.channel_id] = avatar

        await brain.save(
            "avatars",
            json.dumps(avatars.__root__, ensure_ascii=False, default=lambda x: x.dict()).encode("utf-8"),
        )

        await retryable_client.chat_postEphemeral(
            channel=private_metadata.channel_id,
            user=private_metadata.user_id,
            text="アバターを編集しました。",
        )

        await ack()

    @bolt.command("/customize")
    @bolt.command("/_customize")
    async def customize(
        ack: slack_bolt.async_app.AsyncAck,
        client: slack_sdk.web.async_client.AsyncWebClient,
        body: collections.abc.Mapping[str, typing.Any],
    ):
        await ack()

        channel_id = body["channel_id"]
        user_id = body["user_id"]

        restored_avatars = await brain.restore("avatars")
        avatars = (
            Avatars.parse_raw(restored_avatars)
            if restored_avatars is not None
            else Avatars.parse_obj({})
        )

        private_metadata = PrivateMetadata(channel_id=channel_id, user_id=user_id)
        await client.views_open(
            trigger_id=body["trigger_id"],
            view={
                "type": "modal",
                "callback_id": EDIT_AVATAR_CALLBACK_ID,
                "private_metadata": json.dumps(private_metadata.dict(), ensure_ascii=False),
                "title": {"type": "plain_text", "text": "アバターを編集する"},
                "blocks": [
                    {
                        "type": "input",
                        "block_id": NAME_BLOCK_ID,
                        "label": {"type": "plain_text", "text": "名前"},
                        "element": {
                            "type": "plain_text_input",
                            "action_id": NAME_ACTION_ID,
                            "initial_value": avatars[channel_id].name
                            if channel_id in avatars
                            else "猫",
                        },
                    },
                    {
                        "type": "input",
                        "block_id": ICON_URL_BLOCK_ID,
                        "label": {"type": "plain_text", "text": "アイコン画像のURL"},
                        "element": {
                            "type": "plain_text_input",
                            "action_id": ICON_URL_ACTION_ID,
                            "initial_value": avatars[channel_id].icon_url
                            if channel_id in avatars
                            else "https://avatars.githubusercontent.com/u/128691402?v=4",
                        },
                    },
                    {
                        "type": "input",
                        "block_id": INSTRUCTION_BLOCK_ID,
                        "label": {"type": "plain_text", "text": "命令"},
                        "element": {
                            "type": "plain_text_input",
                            "multiline": True,
                            "action_id": INSTRUCTION_ACTION_ID,
                            "initial_value": avatars[channel_id].instruction
                            if channel_id in avatars
                            else "あなたは猫です。\n必ず語尾に「にゃ〜ん」とつけてください。",
                        },
                    },
                    {
                        "type": "input",
                        "block_id": LOCALE_BLOCK_ID,
                        "label": {"type": "plain_text", "text": "言語"},
                        "element": {
                            "type": "static_select",
                            "action_id": LOCALE_ACTION_ID,
                            "options": [
                                {
                                    "text": {"type": "plain_text", "text": bot.slack.i18n.Locale.English},
                                    "value": bot.slack.i18n.Locale.English,
                                },
                                {
                                    "text": {"type": "plain_text", "text": bot.slack.i18n.Locale.Japanese},
                                    "value": bot.slack.i18n.Locale.Japanese,
                                },
                            ],
                            "initial_option": {
                                "text": {
                                    "type": "plain_text",
                                    "text": avatars[channel_id].locale,
                                },
                                "value": avatars[channel_id].locale,
                            }
                            if channel_id in avatars
                            else {
                                "text": {"type": "plain_text", "text": bot.slack.i18n.Locale.Japanese},
                                "value": bot.slack.i18n.Locale.Japanese,
                            },
                        },
                    },
                    {
                        "type": "input",
                        "block_id": TOGGLE_BLOCK_ID,
                        "label": {"type": "plain_text", "text": "設定"},
                        "element": {
                            "type": "radio_buttons",
                            "action_id": TOGGLE_ACTION_ID,
                            "options": [
                                {
                                    "text": {
                                        "type": "plain_text",
                                        "text": "アバターを有効にする",
                                    },
                                    "value": Toggle.On,
                                },
                                {
                                    "text": {
                                        "type": "plain_text",
                                        "text": "アバターを無効にする",
                                    },
                                    "value": Toggle.Off,
                                }
                            ],
                            "initial_option": {
                                "text": {
                                    "type": "plain_text",
                                    "text": "アバターを有効にする"
                                    if avatars[channel_id].enabled == Toggle.On
                                    else "アバターを無効にする",
                                },
                                "value": avatars[channel_id].enabled,
                            }
                            if channel_id in avatars
                            else {
                                "text": {
                                    "type": "plain_text",
                                    "text": "アバターを有効にする",
                                },
                                "value": Toggle.On,
                            },
                        },
                    },
                ],
                "close": {"type": "plain_text", "text": "キャンセル"},
                "submit": {"type": "plain_text", "text": "編集する"},
            },
        )
