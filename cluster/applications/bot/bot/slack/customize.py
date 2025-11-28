import asyncio
import collections.abc
import enum
import json
import typing

import aiohttp
import aiohttp.client_exceptions
import openai.types.chat
import pydantic
import slack_bolt.async_app
import slack_sdk
import slack_sdk.web.async_client

import cortex.brain
import cortex.llm.openai.model
from . import RetryableAsyncWebClient
from .i18n import Locale


class Toggle(enum.StrEnum):
    On = "on"
    Off = "off"


class Avatar(pydantic.BaseModel):
    name: str
    icon_url: str
    instruction: str
    locale: Locale
    model: cortex.llm.openai.model.CompletionModel
    reasoning_effort: openai.types.chat.ChatCompletionReasoningEffort
    enabled: Toggle


class Avatars(
    pydantic.RootModel[collections.abc.MutableMapping[str, Avatar]],
    collections.abc.MutableMapping,
):
    def __len__(self) -> int:
        return self.root.__len__()

    def __iter__(self) -> typing.Iterator[str]:
        return self.root.__iter__()

    def __getitem__(self, __key: str) -> Avatar:
        return self.root.__getitem__(__key)

    def __setitem__(self, __key: str, __value: Avatar) -> None:
        return self.root.__setitem__(__key, __value)

    def __delitem__(self, __key: str) -> None:
        return self.root.__delitem__(__key)


class PrivateMetadata(pydantic.BaseModel):
    channel_id: str
    user_id: str


NAME_BLOCK_ID = "name"
ICON_URL_BLOCK_ID = "icon_url"
INSTRUCTION_BLOCK_ID = "instruction"
LOCALE_BLOCK_ID = "locale"
MODEL_BLOCK_ID = "model"
REASONING_EFFORT_BLOCK_ID = "reasoning_effort"
TOGGLE_BLOCK_ID = "settings"

NAME_ACTION_ID = "name"
ICON_URL_ACTION_ID = "icon_url"
INSTRUCTION_ACTION_ID = "instruction"
LOCALE_ACTION_ID = "locale"
MODEL_ACTION_ID = "model"
REASONING_EFFORT_ACTION_ID = "reasoning_effort"
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
        retryable_client = RetryableAsyncWebClient(client)

        private_metadata = PrivateMetadata.model_validate_json(
            body["view"]["private_metadata"]
        )

        values = body["view"]["state"]["values"]
        avatar = Avatar(
            name=values[NAME_BLOCK_ID][NAME_ACTION_ID]["value"],
            icon_url=values[ICON_URL_BLOCK_ID][ICON_URL_ACTION_ID]["value"],
            instruction=values[INSTRUCTION_BLOCK_ID][INSTRUCTION_ACTION_ID]["value"],
            locale=values[LOCALE_BLOCK_ID][LOCALE_ACTION_ID]["selected_option"][
                "value"
            ],
            model=values[MODEL_BLOCK_ID][MODEL_ACTION_ID]["selected_option"]["value"],
            reasoning_effort=values[REASONING_EFFORT_BLOCK_ID][
                REASONING_EFFORT_ACTION_ID
            ]["selected_option"]["value"],
            enabled=values[TOGGLE_BLOCK_ID][TOGGLE_ACTION_ID]["selected_option"][
                "value"
            ],
        )

        errors = {}
        if len(avatar.name) > OPTION_VALUE_LIMIT:
            errors[NAME_BLOCK_ID] = (
                f"タイトルは{OPTION_VALUE_LIMIT}文字以下にしてください。"
            )

        try:
            async with aiohttp.ClientSession(trust_env=True) as session:
                async with session.get(avatar.icon_url) as response:
                    if response.status != 200:
                        errors[ICON_URL_BLOCK_ID] = (
                            "アイコン画像のURLにアクセスできませんでした。"
                        )
        except aiohttp.InvalidURL:
            errors[ICON_URL_BLOCK_ID] = "アイコン画像のURLが無効です。"
        except (
            aiohttp.ClientConnectionError,  # ECONNREFUSED, EPIPE, ECONNRESET
            aiohttp.client_exceptions.ServerDisconnectedError,
            asyncio.TimeoutError,
        ):
            errors[ICON_URL_BLOCK_ID] = (
                "アイコン画像のURLにアクセスできなかったため、再度お試しください。"
            )

        if errors:
            await ack(response_action="errors", errors=errors)
            return

        restored_avatars = await brain.restore("avatars")
        try:
            avatars = (
                Avatars.model_validate_json(restored_avatars)
                if restored_avatars is not None
                else Avatars.model_validate({})
            )
        except pydantic.ValidationError:
            avatars = Avatars.model_validate({})

        avatars[private_metadata.channel_id] = avatar

        await brain.save(
            "avatars",
            json.dumps(
                avatars.root, ensure_ascii=False, default=lambda x: x.model_dump()
            ).encode("utf-8"),
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
        try:
            avatars = (
                Avatars.model_validate_json(restored_avatars)
                if restored_avatars is not None
                else Avatars.model_validate({})
            )
        except pydantic.ValidationError:
            avatars = Avatars.model_validate({})

        available_languages = [Locale.English, Locale.Japanese]
        available_models = cortex.llm.openai.model.CompletionModel.model_options()
        available_reasoning_effort = typing.get_args(
            typing.get_args(openai.types.chat.ChatCompletionReasoningEffort)[0]
        )
        private_metadata = PrivateMetadata(channel_id=channel_id, user_id=user_id)
        await client.views_open(
            trigger_id=body["trigger_id"],
            view={
                "type": "modal",
                "callback_id": EDIT_AVATAR_CALLBACK_ID,
                "private_metadata": json.dumps(
                    private_metadata.model_dump(), ensure_ascii=False
                ),
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
                                    "text": {"type": "plain_text", "text": language},
                                    "value": language,
                                }
                                for language in available_languages
                            ],
                            "initial_option": {
                                "text": {
                                    "type": "plain_text",
                                    "text": avatars[channel_id].locale
                                    if avatars[channel_id].locale in available_languages
                                    else Locale.Japanese,
                                },
                                "value": avatars[channel_id].locale
                                if avatars[channel_id].locale in available_languages
                                else Locale.Japanese,
                            }
                            if channel_id in avatars
                            else {
                                "text": {"type": "plain_text", "text": Locale.Japanese},
                                "value": Locale.Japanese,
                            },
                        },
                    },
                    {
                        "type": "input",
                        "block_id": MODEL_BLOCK_ID,
                        "label": {"type": "plain_text", "text": "モデル"},
                        "element": {
                            "type": "static_select",
                            "action_id": MODEL_ACTION_ID,
                            "options": [
                                {
                                    "text": {"type": "plain_text", "text": model},
                                    "value": model,
                                }
                                for model in available_models
                            ],
                            "initial_option": {
                                "text": {
                                    "type": "plain_text",
                                    "text": avatars[channel_id].model
                                    if avatars[channel_id].model in available_models
                                    else cortex.llm.openai.model.CompletionModel.GPT4O_ALIAS,
                                },
                                "value": avatars[channel_id].model
                                if avatars[channel_id].model in available_models
                                else cortex.llm.openai.model.CompletionModel.GPT4O_ALIAS,
                            }
                            if channel_id in avatars
                            else {
                                "text": {
                                    "type": "plain_text",
                                    "text": cortex.llm.openai.model.CompletionModel.GPT4O_ALIAS,
                                },
                                "value": cortex.llm.openai.model.CompletionModel.GPT4O_ALIAS,
                            },
                        },
                    },
                    {
                        "type": "input",
                        "block_id": REASONING_EFFORT_BLOCK_ID,
                        "label": {"type": "plain_text", "text": "Reasoning Effort"},
                        "element": {
                            "type": "static_select",
                            "action_id": REASONING_EFFORT_ACTION_ID,
                            "options": [
                                {
                                    "text": {
                                        "type": "plain_text",
                                        "text": reasoning_effort,
                                    },
                                    "value": reasoning_effort,
                                }
                                for reasoning_effort in available_reasoning_effort
                            ],
                            "initial_option": {
                                "text": {
                                    "type": "plain_text",
                                    "text": avatars[channel_id].reasoning_effort
                                    if avatars[channel_id].reasoning_effort
                                    in available_reasoning_effort
                                    else "medium",
                                },
                                "value": avatars[channel_id].reasoning_effort
                                if avatars[channel_id].reasoning_effort
                                in available_reasoning_effort
                                else "medium",
                            }
                            if channel_id in avatars
                            else {
                                "text": {"type": "plain_text", "text": "medium"},
                                "value": "medium",
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
                                },
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
