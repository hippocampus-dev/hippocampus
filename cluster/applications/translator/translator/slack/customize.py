import collections.abc
import enum
import json
import typing

import pydantic
import slack_bolt.async_app
import slack_sdk
import slack_sdk.web.async_client

import cortex.brain
from . import RetryableAsyncWebClient
from .i18n import Locale


class Toggle(enum.StrEnum):
    On = "on"
    Off = "off"


class Translation(pydantic.BaseModel):
    locales: collections.abc.Sequence[Locale] = pydantic.Field(
        default_factory=lambda: [Locale.Japanese, Locale.English]
    )
    collapse: Toggle = Toggle.Off
    enabled: Toggle = Toggle.Off


class Translations(
    pydantic.RootModel[collections.abc.MutableMapping[str, Translation]],
    collections.abc.MutableMapping
):
    def __len__(self) -> int:
        return self.root.__len__()

    def __iter__(self) -> typing.Iterator[str]:
        return self.root.__iter__()

    def __getitem__(self, __key: str) -> Translation:
        return self.root.__getitem__(__key)

    def __setitem__(self, __key: str, __value: Translation) -> None:
        return self.root.__setitem__(__key, __value)

    def __delitem__(self, __key: str) -> None:
        return self.root.__delitem__(__key)


class PrivateMetadata(pydantic.BaseModel):
    channel_id: str
    user_id: str


LANGUAGE_BLOCK_ID = "language"
COLLAPSE_BLOCK_ID = "collapse"
TOGGLE_BLOCK_ID = "translations"

LANGUAGE_ACTION_ID = "language"
COLLAPSE_ACTION_ID = "collapse"
TOGGLE_ACTION_ID = "translations"

EDIT_TRANSLATION_CALLBACK_ID = "edit_translation"


async def register(bolt: slack_bolt.async_app.AsyncApp, brain: cortex.brain.Brain):
    @bolt.view(EDIT_TRANSLATION_CALLBACK_ID)
    async def handle_edit_translations(
        ack: slack_bolt.async_app.AsyncAck,
        client: slack_sdk.web.async_client.AsyncWebClient,
        body: collections.abc.Mapping[str, typing.Any],
    ):
        retryable_client = RetryableAsyncWebClient(client)

        private_metadata = PrivateMetadata.model_validate_json(body["view"]["private_metadata"])

        values = body["view"]["state"]["values"]
        setting = Translation(
            locales=[
                selected_option["value"]
                for selected_option in values[LANGUAGE_BLOCK_ID][LANGUAGE_ACTION_ID]["selected_options"]
            ],
            collapse=values[COLLAPSE_BLOCK_ID][COLLAPSE_ACTION_ID]["selected_option"]["value"],
            enabled=values[TOGGLE_BLOCK_ID][TOGGLE_ACTION_ID]["selected_option"]["value"],
        )

        errors = {}

        if errors:
            await ack(response_action="errors", errors=errors)
            return

        restored_settings = await brain.restore("translations")
        translations = (
            Translations.model_validate_json(restored_settings)
            if restored_settings is not None
            else Translations.model_validate({})
        )

        translations[private_metadata.channel_id] = setting

        await brain.save(
            "translations",
            json.dumps(translations.root, ensure_ascii=False, default=lambda x: x.model_dump()).encode("utf-8"),
        )

        await retryable_client.chat_postEphemeral(
            channel=private_metadata.channel_id,
            user=private_metadata.user_id,
            text="設定を更新しました。",
        )

        await ack()

    @bolt.command("/translation")
    @bolt.command("/_translation")
    async def translation(
        ack: slack_bolt.async_app.AsyncAck,
        client: slack_sdk.web.async_client.AsyncWebClient,
        body: collections.abc.Mapping[str, typing.Any],
    ):
        await ack()

        channel_id = body["channel_id"]
        user_id = body["user_id"]

        restored_translations = await brain.restore("translations")
        translations = (
            Translations.model_validate_json(restored_translations)
            if restored_translations is not None
            else Translations.model_validate({})
        )

        private_metadata = PrivateMetadata(channel_id=channel_id, user_id=user_id)
        await client.views_open(
            trigger_id=body["trigger_id"],
            view={
                "type": "modal",
                "callback_id": EDIT_TRANSLATION_CALLBACK_ID,
                "private_metadata": json.dumps(private_metadata.model_dump(), ensure_ascii=False),
                "title": {"type": "plain_text", "text": "設定を更新する"},
                "blocks": [
                    {
                        "type": "input",
                        "block_id": LANGUAGE_BLOCK_ID,
                        "label": {"type": "plain_text", "text": "言語"},
                        "element": {
                            "type": "multi_static_select",
                            "action_id": LANGUAGE_ACTION_ID,
                            "options": [
                                {
                                    "text": {"type": "plain_text", "text": Locale.English},
                                    "value": Locale.English,
                                },
                                {
                                    "text": {"type": "plain_text", "text": Locale.Japanese},
                                    "value": Locale.Japanese,
                                },
                                {
                                    "text": {"type": "plain_text", "text": Locale.Vietnamese},
                                    "value": Locale.Vietnamese,
                                },
                            ],
                            "initial_options": [
                                {
                                    "text": {
                                        "type": "plain_text",
                                        "text": locale,
                                    },
                                    "value": locale,
                                }
                                for locale in translations[channel_id].locales
                            ]
                            if channel_id in translations
                            else [{
                                "text": {"type": "plain_text", "text": Locale.Japanese},
                                "value": Locale.Japanese,
                            }, {
                                "text": {"type": "plain_text", "text": Locale.English},
                                "value": Locale.English,
                            }],
                        },
                    },
                    {
                        "type": "input",
                        "block_id": COLLAPSE_BLOCK_ID,
                        "label": {"type": "plain_text", "text": "折りたたみ"},
                        "element": {
                            "type": "radio_buttons",
                            "action_id": COLLAPSE_ACTION_ID,
                            "options": [
                                {
                                    "text": {
                                        "type": "plain_text",
                                        "text": "折りたたみを有効にする",
                                    },
                                    "value": Toggle.On,
                                },
                                {
                                    "text": {
                                        "type": "plain_text",
                                        "text": "折りたたみを無効にする",
                                    },
                                    "value": Toggle.Off,
                                }
                            ],
                            "initial_option": {
                                "text": {
                                    "type": "plain_text",
                                    "text": "折りたたみを有効にする"
                                    if translations[channel_id].collapse == Toggle.On
                                    else "折りたたみを無効にする",
                                },
                                "value": translations[channel_id].collapse,
                            }
                            if channel_id in translations
                            else {
                                "text": {
                                    "type": "plain_text",
                                    "text": "折りたたみを無効にする",
                                },
                                "value": Toggle.Off,
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
                                        "text": "翻訳を有効にする",
                                    },
                                    "value": Toggle.On,
                                },
                                {
                                    "text": {
                                        "type": "plain_text",
                                        "text": "翻訳を無効にする",
                                    },
                                    "value": Toggle.Off,
                                }
                            ],
                            "initial_option": {
                                "text": {
                                    "type": "plain_text",
                                    "text": "翻訳を有効にする"
                                    if translations[channel_id].enabled == Toggle.On
                                    else "翻訳を無効にする",
                                },
                                "value": translations[channel_id].enabled,
                            }
                            if channel_id in translations
                            else {
                                "text": {
                                    "type": "plain_text",
                                    "text": "翻訳を有効にする",
                                },
                                "value": Toggle.On,
                            },
                        },
                    },
                ],
                "close": {"type": "plain_text", "text": "キャンセル"},
                "submit": {"type": "plain_text", "text": "更新する"},
            },
        )
