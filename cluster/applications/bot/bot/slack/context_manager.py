import collections.abc
import json
import typing

import pydantic
import slack_bolt.async_app
import slack_bolt.context.ack.async_ack
import slack_bolt.context.respond.async_respond
import slack_sdk.web.async_client
import slack_sdk.web.async_slack_response

import bot
import cortex.brain


class Context(pydantic.BaseModel):
    user: str
    title: str
    body: str


class Contexts(
    pydantic.RootModel[collections.abc.MutableMapping[str, Context]],
    collections.abc.MutableMapping,
):
    def __len__(self) -> int:
        return self.root.__len__()

    def __iter__(self) -> typing.Iterator[str]:
        return self.root.__iter__()

    def __getitem__(self, __key: str) -> Context:
        return self.root.__getitem__(__key)

    def __setitem__(self, __key: str, __value: Context) -> None:
        return self.root.__setitem__(__key, __value)

    def __delitem__(self, __key: str) -> None:
        return self.root.__delitem__(__key)


class PrivateMetadata(pydantic.BaseModel):
    channel_id: str
    user_id: str
    selected_context_title: str | None = None


def format_markdown(context: Context, body_size_limit: int = 100) -> str:
    return (
        f"*{context.title}* (<@{context.user}>)\n"
        "\n"
        f"{context.body[:body_size_limit] + '...' if len(context.body) > body_size_limit else context.body}"
    )


_TITLE_BLOCK_ID = "title"
_TITLE_ACTION_ID = "title"
_BODY_BLOCK_ID = "context"
_BODY_ACTION_ID = "input"

_ADD_CONFIRM_CALLBACK_ID = "add-confirm"
_EDIT_CONFIRM_CALLBACK_ID = "edit-confirm"
_DELETE_CONFIRM_CALLBACK_ID = "delete-confirm"
_USE_CONFIRM_CALLBACK_ID = "use-confirm"  # Used by list, select

_EDIT_CONTEXT_CALLBACK_ID = "edit-context"
_EDIT_CONTEXT_BLOCK_ID = "edit"
_EDIT_CONTEXT_ACTION_ID = "edit"

_DELETE_CONTEXT_CALLBACK_ID = "delete-context"
_DELETE_CONTEXT_BLOCK_ID = "delete"
_DELETE_CONTEXT_ACTION_ID = "delete"

_LIST_CONTEXT_CALLBACK_ID = "list-context"
_LIST_CONTEXT_BLOCK_ID = "list"
_LIST_CONTEXT_ACTION_ID = "list"

_SELECT_CONTEXT_CALLBACK_ID = "select-context"
_SELECT_CONTEXT_BLOCK_ID = "select"
_SELECT_CONTEXT_ACTION_ID = "select"

_OPTION_TEXT_LIMIT = 75
_OPTION_VALUE_LIMIT = 150


async def register(bolt: slack_bolt.async_app.AsyncApp, brain: cortex.brain.Brain):
    @bolt.view(_ADD_CONFIRM_CALLBACK_ID)
    async def handle_add_context(
        ack: slack_bolt.context.ack.async_ack.AsyncAck,
        client: slack_sdk.web.async_client.AsyncWebClient,
        body: collections.abc.Mapping[str, typing.Any],
    ):
        retryable_client = bot.slack.RetryableAsyncWebClient(client)

        private_metadata = PrivateMetadata.model_validate_json(
            body["view"]["private_metadata"]
        )

        values = body["view"]["state"]["values"]
        context = Context(
            user=body["user"]["id"],
            title=values[_TITLE_BLOCK_ID][_TITLE_ACTION_ID]["value"],
            body=values[_BODY_BLOCK_ID][_BODY_ACTION_ID]["value"],
        )

        errors = {}
        if len(context.title) > _OPTION_VALUE_LIMIT:
            errors[_TITLE_BLOCK_ID] = (
                f"タイトルは{_OPTION_VALUE_LIMIT}文字以下にしてください。"
            )

        if errors:
            await ack(response_action="errors", errors=errors)
            return

        restored_contexts = await brain.restore("contexts")
        contexts = (
            Contexts.model_validate_json(restored_contexts)
            if restored_contexts
            else Contexts.model_validate({})
        )

        if context.title in contexts:
            await retryable_client.chat_postEphemeral(
                channel=private_metadata.channel_id,
                user=private_metadata.user_id,
                text=f"`{context.title}` は既に存在します。",
            )
            return

        contexts[context.title] = context

        await brain.save(
            "contexts",
            json.dumps(
                contexts.root, ensure_ascii=False, default=lambda x: x.model_dump()
            ).encode("utf-8"),
        )

        await retryable_client.chat_postEphemeral(
            channel=private_metadata.channel_id,
            user=private_metadata.user_id,
            text=f"`{context.title}` を追加しました。",
        )

        await ack()

    @bolt.view(_EDIT_CONFIRM_CALLBACK_ID)
    async def handle_edit_confirm(
        ack: slack_bolt.context.ack.async_ack.AsyncAck,
        client: slack_sdk.web.async_client.AsyncWebClient,
        body: collections.abc.Mapping[str, typing.Any],
    ):
        retryable_client = bot.slack.RetryableAsyncWebClient(client)

        private_metadata = PrivateMetadata.model_validate_json(
            body["view"]["private_metadata"]
        )

        values = body["view"]["state"]["values"]
        context = Context(
            user=body["user"]["id"],
            title=values[_TITLE_BLOCK_ID][_TITLE_ACTION_ID]["value"],
            body=values[_BODY_BLOCK_ID][_BODY_ACTION_ID]["value"],
        )

        errors = {}
        if len(context.title) > _OPTION_VALUE_LIMIT:
            errors[_TITLE_BLOCK_ID] = (
                f"タイトルは{_OPTION_VALUE_LIMIT}文字以下にしてください。"
            )

        if errors:
            await ack(response_action="errors", errors=errors)
            return

        restored_contexts = await brain.restore("contexts")
        contexts = (
            Contexts.model_validate_json(restored_contexts)
            if restored_contexts
            else Contexts.model_validate({})
        )

        del contexts[private_metadata.selected_context_title]

        if context.title in contexts:
            await retryable_client.chat_postEphemeral(
                channel=private_metadata.channel_id,
                user=private_metadata.user_id,
                text=f"`{context.title}` は既に存在します。",
            )
            return

        contexts[context.title] = context

        await brain.save(
            "contexts",
            json.dumps(
                contexts.root, ensure_ascii=False, default=lambda x: x.model_dump()
            ).encode("utf-8"),
        )

        await retryable_client.chat_postEphemeral(
            channel=private_metadata.channel_id,
            user=private_metadata.user_id,
            text=f"`{context.title}` を編集しました。",
        )

        await ack()

    @bolt.view(_DELETE_CONFIRM_CALLBACK_ID)
    async def handle_delete_confirm(
        ack: slack_bolt.context.ack.async_ack.AsyncAck,
        client: slack_sdk.web.async_client.AsyncWebClient,
        body: collections.abc.Mapping[str, typing.Any],
    ):
        await ack()

        retryable_client = bot.slack.RetryableAsyncWebClient(client)

        private_metadata = PrivateMetadata.model_validate_json(
            body["view"]["private_metadata"]
        )

        title = body["view"]["blocks"][0]["block_id"]

        restored_contexts = await brain.restore("contexts")
        contexts = (
            Contexts.model_validate_json(restored_contexts)
            if restored_contexts
            else Contexts.model_validate({})
        )

        del contexts[private_metadata.selected_context_title]

        await brain.save(
            "contexts",
            json.dumps(
                contexts.root, ensure_ascii=False, default=lambda x: x.model_dump()
            ).encode("utf-8"),
        )

        await retryable_client.chat_postEphemeral(
            channel=private_metadata.channel_id,
            user=private_metadata.user_id,
            text=f"`{title}` を削除しました。",
        )

    @bolt.view(_USE_CONFIRM_CALLBACK_ID)
    async def handle_use_confirm(
        ack: slack_bolt.context.ack.async_ack.AsyncAck,
        client: slack_sdk.web.async_client.AsyncWebClient,
        body: collections.abc.Mapping[str, typing.Any],
    ):
        await ack()

        retryable_client = bot.slack.RetryableAsyncWebClient(client)

        private_metadata = PrivateMetadata.model_validate_json(
            body["view"]["private_metadata"]
        )

        title = body["view"]["blocks"][0]["block_id"]

        restored_contexts = await brain.restore("contexts")
        contexts = (
            Contexts.model_validate_json(restored_contexts)
            if restored_contexts
            else Contexts.model_validate({})
        )

        context = contexts[title]

        await retryable_client.chat_postEphemeral(
            channel=private_metadata.channel_id,
            user=private_metadata.user_id,
            text=f"`{context.title}` のコンテキストを使います。"
            "以下のスレッド内で会話を始めてください。",
        )
        await retryable_client.chat_postMessage(
            channel=private_metadata.channel_id, text=context.body
        )

    # Add

    async def add_context(
        ack: slack_bolt.context.ack.async_ack.AsyncAck,
        client: slack_sdk.web.async_client.AsyncWebClient,
        retryable_client: slack_sdk.web.async_client.AsyncWebClient,
        body: collections.abc.Mapping[str, typing.Any],
    ):
        await ack()

        private_metadata = PrivateMetadata(
            channel_id=body["channel_id"], user_id=body["user_id"]
        )
        await retryable_client.views_open(
            trigger_id=body["trigger_id"],
            view={
                "type": "modal",
                "callback_id": _ADD_CONFIRM_CALLBACK_ID,
                "private_metadata": json.dumps(
                    private_metadata.model_dump(), ensure_ascii=False
                ),
                "title": {"type": "plain_text", "text": "コンテキストを追加する"},
                "blocks": [
                    {
                        "type": "input",
                        "block_id": _TITLE_BLOCK_ID,
                        "label": {"type": "plain_text", "text": "タイトル"},
                        "element": {
                            "type": "plain_text_input",
                            "action_id": _TITLE_ACTION_ID,
                            "placeholder": {"type": "plain_text", "text": "猫"},
                        },
                    },
                    {
                        "type": "input",
                        "block_id": _BODY_BLOCK_ID,
                        "label": {"type": "plain_text", "text": "コンテキスト"},
                        "element": {
                            "type": "plain_text_input",
                            "multiline": True,
                            "action_id": _BODY_ACTION_ID,
                            "placeholder": {
                                "type": "plain_text",
                                "text": "あなたは猫です。\n必ず語尾に「にゃ〜ん」とつけてください。",
                            },
                        },
                    },
                ],
                "close": {"type": "plain_text", "text": "キャンセル"},
                "submit": {"type": "plain_text", "text": "追加する"},
            },
        )

    # Edit

    async def edit_context(
        ack: slack_bolt.context.ack.async_ack.AsyncAck,
        client: slack_sdk.web.async_client.AsyncWebClient,
        retryable_client: slack_sdk.web.async_client.AsyncWebClient,
        body: collections.abc.Mapping[str, typing.Any],
    ):
        await ack()

        restored_contexts = await brain.restore("contexts")
        contexts = (
            Contexts.model_validate_json(restored_contexts)
            if restored_contexts
            else Contexts.model_validate({})
        )

        private_metadata = PrivateMetadata(
            channel_id=body["channel_id"], user_id=body["user_id"]
        )
        await retryable_client.views_open(
            trigger_id=body["trigger_id"],
            view={
                "type": "modal",
                "callback_id": _EDIT_CONTEXT_CALLBACK_ID,
                "private_metadata": json.dumps(
                    private_metadata.model_dump(), ensure_ascii=False
                ),
                "title": {"type": "plain_text", "text": "コンテキストを編集する"},
                "blocks": [
                    {
                        "type": "input",
                        "block_id": _EDIT_CONTEXT_BLOCK_ID,
                        "label": {"type": "plain_text", "text": "タイトル"},
                        "element": {
                            "type": "static_select",
                            "action_id": _EDIT_CONTEXT_ACTION_ID,
                            "options": [
                                {
                                    "text": {"type": "plain_text", "text": title},
                                    "value": title,
                                }
                                for title, context in contexts.items()
                                if context.user == private_metadata.user_id
                            ],
                            "placeholder": {
                                "type": "plain_text",
                                "text": "タイトルを入力してください",
                            },
                        },
                    },
                ],
                "close": {"type": "plain_text", "text": "キャンセル"},
                "submit": {
                    "type": "plain_text",
                    "text": ":warning: 編集する",
                    "emoji": True,
                },
            },
        )

    @bolt.view(_EDIT_CONTEXT_CALLBACK_ID)
    async def handle_edit_context(
        ack: slack_bolt.context.ack.async_ack.AsyncAck,
        client: slack_sdk.web.async_client.AsyncWebClient,
        body: collections.abc.Mapping[str, typing.Any],
    ):
        await ack()

        retryable_client = bot.slack.RetryableAsyncWebClient(client)

        private_metadata = PrivateMetadata.model_validate_json(
            body["view"]["private_metadata"]
        )

        values = body["view"]["state"]["values"]
        title = values[_EDIT_CONTEXT_BLOCK_ID][_EDIT_CONTEXT_ACTION_ID][
            "selected_option"
        ]["value"]

        restored_contexts = await brain.restore("contexts")
        contexts = (
            Contexts.model_validate_json(restored_contexts)
            if restored_contexts
            else Contexts.model_validate({})
        )

        context = contexts[title]
        if context.user != private_metadata.user_id:
            await retryable_client.chat_postEphemeral(
                channel=private_metadata.channel_id,
                user=private_metadata.user_id,
                text=f"`{context.title}` を編集できませんでした。",
            )
            return

        private_metadata.selected_context_title = context.title

        await retryable_client.views_open(
            trigger_id=body["trigger_id"],
            view={
                "type": "modal",
                "callback_id": _EDIT_CONFIRM_CALLBACK_ID,
                "private_metadata": json.dumps(
                    private_metadata.model_dump(), ensure_ascii=False
                ),
                "title": {"type": "plain_text", "text": "コンテキストを編集する"},
                "blocks": [
                    {
                        "type": "input",
                        "block_id": _TITLE_BLOCK_ID,
                        "label": {"type": "plain_text", "text": "タイトル"},
                        "element": {
                            "type": "plain_text_input",
                            "action_id": _TITLE_ACTION_ID,
                            "initial_value": context.title,
                        },
                    },
                    {
                        "type": "input",
                        "block_id": _BODY_BLOCK_ID,
                        "label": {"type": "plain_text", "text": "コンテキスト"},
                        "element": {
                            "type": "plain_text_input",
                            "multiline": True,
                            "action_id": _BODY_ACTION_ID,
                            "initial_value": context.body,
                        },
                    },
                ],
                "close": {"type": "plain_text", "text": "キャンセル"},
                "submit": {
                    "type": "plain_text",
                    "text": ":warning: 編集する",
                    "emoji": True,
                },
            },
        )

    # Delete

    async def delete_context(
        ack: slack_bolt.context.ack.async_ack.AsyncAck,
        client: slack_sdk.web.async_client.AsyncWebClient,
        retryable_client: slack_sdk.web.async_client.AsyncWebClient,
        body: collections.abc.Mapping[str, typing.Any],
    ):
        await ack()

        restored_contexts = await brain.restore("contexts")
        contexts = (
            Contexts.model_validate_json(restored_contexts)
            if restored_contexts
            else Contexts.model_validate({})
        )

        private_metadata = PrivateMetadata(
            channel_id=body["channel_id"], user_id=body["user_id"]
        )
        options = [
            {
                "text": {
                    "type": "plain_text",
                    "text": title[: _OPTION_TEXT_LIMIT - 3] + "..."
                    if len(title) > _OPTION_TEXT_LIMIT
                    else title,
                },
                "value": title,
            }
            for title, context in contexts.items()
            if context.user == private_metadata.user_id
        ]
        if len(options) == 0:
            await retryable_client.chat_postEphemeral(
                channel=private_metadata.channel_id,
                user=private_metadata.user_id,
                text="削除するコンテキストがありません。",
            )
            return

        await retryable_client.views_open(
            trigger_id=body["trigger_id"],
            view={
                "type": "modal",
                "callback_id": _DELETE_CONTEXT_CALLBACK_ID,
                "private_metadata": json.dumps(
                    private_metadata.model_dump(), ensure_ascii=False
                ),
                "title": {"type": "plain_text", "text": "コンテキストを削除する"},
                "blocks": [
                    {
                        "type": "input",
                        "block_id": _DELETE_CONTEXT_BLOCK_ID,
                        "label": {"type": "plain_text", "text": "タイトル"},
                        "element": {
                            "type": "static_select",
                            "action_id": _DELETE_CONTEXT_ACTION_ID,
                            "options": options,
                            "placeholder": {
                                "type": "plain_text",
                                "text": "タイトルを入力してください",
                            },
                        },
                    },
                ],
                "close": {"type": "plain_text", "text": "キャンセル"},
                "submit": {
                    "type": "plain_text",
                    "text": ":warning: 削除する",
                    "emoji": True,
                },
            },
        )

    @bolt.view(_DELETE_CONTEXT_CALLBACK_ID)
    async def handle_delete_context(
        ack: slack_bolt.context.ack.async_ack.AsyncAck,
        client: slack_sdk.web.async_client.AsyncWebClient,
        body: collections.abc.Mapping[str, typing.Any],
    ):
        await ack()

        retryable_client = bot.slack.RetryableAsyncWebClient(client)

        private_metadata = PrivateMetadata.model_validate_json(
            body["view"]["private_metadata"]
        )

        values = body["view"]["state"]["values"]
        title = values[_DELETE_CONTEXT_BLOCK_ID][_DELETE_CONTEXT_ACTION_ID][
            "selected_option"
        ]["value"]

        restored_contexts = await brain.restore("contexts")
        contexts = (
            Contexts.model_validate_json(restored_contexts)
            if restored_contexts
            else Contexts.model_validate({})
        )

        context = contexts[title]
        if context.user != private_metadata.user_id:
            await retryable_client.chat_postEphemeral(
                channel=private_metadata.channel_id,
                user=private_metadata.user_id,
                text=f"`{context.title}` を削除できませんでした。",
            )
            return

        private_metadata.selected_context_title = context.title

        # Prepare a Confirm procedure for use_context compatibility.
        await retryable_client.views_open(
            trigger_id=body["trigger_id"],
            view={
                "type": "modal",
                "callback_id": _DELETE_CONFIRM_CALLBACK_ID,
                "private_metadata": json.dumps(
                    private_metadata.model_dump(), ensure_ascii=False
                ),
                "title": {"type": "plain_text", "text": "これを削除しますか？"},
                "blocks": [
                    {
                        "type": "section",
                        "block_id": context.title,
                        "text": {
                            "type": "mrkdwn",
                            "text": format_markdown(context),
                        },
                    },
                ],
                "close": {"type": "plain_text", "text": "キャンセル"},
                "submit": {"type": "plain_text", "text": ":warning: 確認する"},
            },
        )

    # List

    async def list_context(
        ack: slack_bolt.context.ack.async_ack.AsyncAck,
        client: slack_sdk.web.async_client.AsyncWebClient,
        retryable_client: slack_sdk.web.async_client.AsyncWebClient,
        body: collections.abc.Mapping[str, typing.Any],
    ):
        await ack()

        restored_contexts = await brain.restore("contexts")
        contexts = (
            Contexts.model_validate_json(restored_contexts)
            if restored_contexts
            else Contexts.model_validate({})
        )

        blocks = []
        for context in contexts.values():
            blocks.append(
                {
                    "type": "section",
                    "block_id": context.title,
                    "text": {
                        "type": "mrkdwn",
                        "text": format_markdown(context),
                    },
                },
            )
            blocks.append(
                {
                    "type": "actions",
                    "elements": [
                        {
                            "type": "button",
                            "text": {
                                "type": "plain_text",
                                "text": "これを利用する",
                            },
                            "value": context.title,
                            "action_id": _LIST_CONTEXT_ACTION_ID,
                        },
                    ],
                },
            )

        private_metadata = PrivateMetadata(
            channel_id=body["channel_id"], user_id=body["user_id"]
        )
        await retryable_client.views_open(
            trigger_id=body["trigger_id"],
            view={
                "type": "modal",
                "callback_id": _LIST_CONTEXT_CALLBACK_ID,
                "private_metadata": json.dumps(
                    private_metadata.model_dump(), ensure_ascii=False
                ),
                "title": {"type": "plain_text", "text": "コンテキストを利用する"},
                "blocks": blocks,
                "close": {"type": "plain_text", "text": "キャンセル"},
            },
        )

    @bolt.action(_LIST_CONTEXT_ACTION_ID)
    async def handle_list_context_button(
        ack: slack_bolt.context.ack.async_ack.AsyncAck,
        client: slack_sdk.web.async_client.AsyncWebClient,
        body: collections.abc.Mapping[str, typing.Any],
    ):
        await ack()

        retryable_client = bot.slack.RetryableAsyncWebClient(client)

        title = body["actions"][0]["value"]

        restored_contexts = await brain.restore("contexts")
        contexts = (
            Contexts.model_validate_json(restored_contexts)
            if restored_contexts
            else Contexts.model_validate({})
        )

        context = contexts[title]

        # Prepare a Confirm procedure because the modal cannot be closed with a button click action.
        await retryable_client.views_update(
            view_id=body["container"]["view_id"],
            view={
                "type": "modal",
                "callback_id": _USE_CONFIRM_CALLBACK_ID,
                "private_metadata": body["view"]["private_metadata"],
                "title": {"type": "plain_text", "text": "これを利用しますか？"},
                "blocks": [
                    {
                        "type": "section",
                        "block_id": context.title,
                        "text": {
                            "type": "mrkdwn",
                            "text": format_markdown(context),
                        },
                    },
                ],
                "close": {"type": "plain_text", "text": "キャンセル"},
                "submit": {"type": "plain_text", "text": "確認する"},
            },
        )

    # Select

    async def select_context(
        ack: slack_bolt.context.ack.async_ack.AsyncAck,
        client: slack_sdk.web.async_client.AsyncWebClient,
        retryable_client: slack_sdk.web.async_client.AsyncWebClient,
        body: collections.abc.Mapping[str, typing.Any],
    ):
        await ack()

        restored_contexts = await brain.restore("contexts")
        contexts = (
            Contexts.model_validate_json(restored_contexts)
            if restored_contexts
            else Contexts.model_validate({})
        )

        private_metadata = PrivateMetadata(
            channel_id=body["channel_id"], user_id=body["user_id"]
        )
        await retryable_client.views_open(
            trigger_id=body["trigger_id"],
            view={
                "type": "modal",
                "callback_id": _SELECT_CONTEXT_CALLBACK_ID,
                "private_metadata": json.dumps(
                    private_metadata.model_dump(), ensure_ascii=False
                ),
                "title": {"type": "plain_text", "text": "コンテキストを利用する"},
                "blocks": [
                    {
                        "type": "input",
                        "block_id": _SELECT_CONTEXT_BLOCK_ID,
                        "label": {"type": "plain_text", "text": "タイトル"},
                        "element": {
                            "type": "static_select",
                            "action_id": _SELECT_CONTEXT_ACTION_ID,
                            "options": [
                                {
                                    "text": {
                                        "type": "plain_text",
                                        "text": title[: _OPTION_TEXT_LIMIT - 3] + "..."
                                        if len(title) > _OPTION_TEXT_LIMIT
                                        else title,
                                    },
                                    "value": title,
                                }
                                for title in contexts.keys()
                            ],
                            "placeholder": {
                                "type": "plain_text",
                                "text": "タイトルを入力してください",
                            },
                        },
                    },
                ],
                "close": {"type": "plain_text", "text": "キャンセル"},
                "submit": {"type": "plain_text", "text": "利用する"},
            },
        )

    @bolt.view(_SELECT_CONTEXT_CALLBACK_ID)
    async def handle_select_context(
        ack: slack_bolt.context.ack.async_ack.AsyncAck,
        client: slack_sdk.web.async_client.AsyncWebClient,
        body: collections.abc.Mapping[str, typing.Any],
    ):
        await ack()

        retryable_client = bot.slack.RetryableAsyncWebClient(client)

        values = body["view"]["state"]["values"]
        title = values[_SELECT_CONTEXT_BLOCK_ID][_SELECT_CONTEXT_ACTION_ID][
            "selected_option"
        ]["value"]

        restored_contexts = await brain.restore("contexts")
        contexts = (
            Contexts.model_validate_json(restored_contexts)
            if restored_contexts
            else Contexts.model_validate({})
        )

        context = contexts[title]

        # Prepare a Confirm procedure for list_context compatibility.
        await retryable_client.views_open(
            trigger_id=body["trigger_id"],
            view={
                "type": "modal",
                "callback_id": _USE_CONFIRM_CALLBACK_ID,
                "private_metadata": body["view"]["private_metadata"],
                "title": {"type": "plain_text", "text": "これを利用しますか？"},
                "blocks": [
                    {
                        "type": "section",
                        "block_id": context.title,
                        "text": {
                            "type": "mrkdwn",
                            "text": format_markdown(context),
                        },
                    },
                ],
                "close": {"type": "plain_text", "text": "キャンセル"},
                "submit": {"type": "plain_text", "text": "利用する"},
            },
        )

    @bolt.command("/context")
    @bolt.command("/_context")
    async def context(
        ack: slack_bolt.context.ack.async_ack.AsyncAck,
        client: slack_sdk.web.async_client.AsyncWebClient,
        body: collections.abc.Mapping[str, typing.Any],
    ):
        await ack()

        retryable_client = bot.slack.RetryableAsyncWebClient(client)

        match body["text"]:
            case "add":
                await add_context(ack, client, retryable_client, body)
            case "edit":
                await edit_context(ack, client, retryable_client, body)
            case "delete":
                await delete_context(ack, client, retryable_client, body)
            case "list":
                await list_context(ack, client, retryable_client, body)
            case "select":
                await select_context(ack, client, retryable_client, body)
            case _:
                await retryable_client.chat_postEphemeral(
                    channel=body["channel_id"],
                    user=body["user_id"],
                    text="`/context list`, `/context select` または  `/context add`, `/context edit`, `/context delete` と入力してください。",
                )
