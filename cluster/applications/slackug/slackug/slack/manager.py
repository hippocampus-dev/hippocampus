import collections.abc
import enum
import json
import typing

import pydantic
import slack_bolt.async_app
import slack_bolt.context.ack.async_ack
import slack_bolt.context.respond.async_respond
import slack_sdk.web.async_client
import slack_sdk.web.async_slack_response

from .. import brain
from .. import slack


class Toggle(enum.StrEnum):
    On = "on"
    Off = "off"


class UserGroup(pydantic.BaseModel):
    name: str
    users: list[str]
    channel_id: str | None = None


class UserGroups(pydantic.RootModel[collections.abc.MutableMapping[str, UserGroup]], collections.abc.MutableMapping):
    def __len__(self) -> int:
        return self.root.__len__()

    def __iter__(self) -> typing.Iterator[str]:
        return self.root.__iter__()

    def __getitem__(self, __key: str) -> UserGroup:
        return self.root.__getitem__(__key)

    def __setitem__(self, __key: str, __value: UserGroup) -> None:
        return self.root.__setitem__(__key, __value)

    def __delitem__(self, __key: str) -> None:
        return self.root.__delitem__(__key)


class PrivateMetadata(pydantic.BaseModel):
    channel_id: str
    user_id: str
    selected_group_name: str | None = None
    list_page: int = 0


def format_markdown(group: UserGroup) -> str:
    users_text = ", ".join([f"<@{user}>" for user in group.users])
    return f"*{group.name}*\n\n{users_text}"


NAME_BLOCK_ID = "name"
NAME_ACTION_ID = "name"
USERS_BLOCK_ID = "users"
USERS_ACTION_ID = "users"
LOCAL_BLOCK_ID = "local"
LOCAL_ACTION_ID = "local"

ADD_CONFIRM_CALLBACK_ID = "add-confirm"
EDIT_CONFIRM_CALLBACK_ID = "edit-confirm"
DELETE_CONFIRM_CALLBACK_ID = "delete-confirm"
USE_CONFIRM_CALLBACK_ID = "use-confirm"

EDIT_GROUP_CALLBACK_ID = "edit-group"
EDIT_GROUP_BLOCK_ID = "edit"
EDIT_GROUP_ACTION_ID = "edit"

DELETE_GROUP_CALLBACK_ID = "delete-group"
DELETE_GROUP_BLOCK_ID = "delete"
DELETE_GROUP_ACTION_ID = "delete"

LIST_GROUP_CALLBACK_ID = "list-group"
LIST_GROUP_BLOCK_ID = "list"
LIST_GROUP_ACTION_ID = "list"
LIST_GROUP_NEXT_ACTION_ID = "list-next"
LIST_GROUP_PREV_ACTION_ID = "list-prev"

ITEMS_PER_PAGE = 50

SELECT_GROUP_CALLBACK_ID = "select-group"
SELECT_GROUP_BLOCK_ID = "select"
SELECT_GROUP_ACTION_ID = "select"

OPTION_TEXT_LIMIT = 75
OPTION_VALUE_LIMIT = 150


async def register(bolt: slack_bolt.async_app.AsyncApp, brain: brain.Brain):
    @bolt.view(ADD_CONFIRM_CALLBACK_ID)
    async def handle_add_group(
        ack: slack_bolt.context.ack.async_ack.AsyncAck,
        client: slack_sdk.web.async_client.AsyncWebClient,
        body: collections.abc.Mapping[str, typing.Any],
    ):
        retryable_client = slack.RetryableAsyncWebClient(client)

        private_metadata = PrivateMetadata.model_validate_json(body["view"]["private_metadata"])

        values = body["view"]["state"]["values"]

        is_local = values[LOCAL_BLOCK_ID][LOCAL_ACTION_ID]["selected_option"]["value"] == Toggle.On

        group = UserGroup(
            name=values[NAME_BLOCK_ID][NAME_ACTION_ID]["value"],
            users=values[USERS_BLOCK_ID][USERS_ACTION_ID]["selected_conversations"],
            channel_id=private_metadata.channel_id if is_local else None,
        )

        errors = {}
        if len(group.name) > OPTION_VALUE_LIMIT:
            errors[NAME_BLOCK_ID] = f"ユーザグループ名は{OPTION_VALUE_LIMIT}文字以下にしてください。"

        if errors:
            await ack(response_action="errors", errors=errors)
            return

        restored_groups = await brain.restore("usergroups")
        groups = UserGroups.model_validate_json(restored_groups) if restored_groups else UserGroups.model_validate({})

        if group.name in groups:
            await retryable_client.chat_postEphemeral(
                channel=private_metadata.channel_id,
                user=private_metadata.user_id,
                text=f"`{group.name}` は既に存在します。",
            )
            return

        groups[group.name] = group

        await brain.save(
            "usergroups",
            json.dumps(groups.root, ensure_ascii=False, default=lambda x: x.model_dump()).encode("utf-8"),
        )

        await retryable_client.chat_postEphemeral(
            channel=private_metadata.channel_id,
            user=private_metadata.user_id,
            text=f"`{group.name}` を追加しました。",
        )

        await ack()

    @bolt.view(EDIT_CONFIRM_CALLBACK_ID)
    async def handle_edit_confirm(
        ack: slack_bolt.context.ack.async_ack.AsyncAck,
        client: slack_sdk.web.async_client.AsyncWebClient,
        body: collections.abc.Mapping[str, typing.Any],
    ):
        retryable_client = slack.RetryableAsyncWebClient(client)

        private_metadata = PrivateMetadata.model_validate_json(body["view"]["private_metadata"])

        values = body["view"]["state"]["values"]

        is_local = values[LOCAL_BLOCK_ID][LOCAL_ACTION_ID]["selected_option"]["value"] == Toggle.On

        group = UserGroup(
            name=values[NAME_BLOCK_ID][NAME_ACTION_ID]["value"],
            users=values[USERS_BLOCK_ID][USERS_ACTION_ID]["selected_conversations"],
            channel_id=private_metadata.channel_id if is_local else None,
        )

        errors = {}
        if len(group.name) > OPTION_VALUE_LIMIT:
            errors[NAME_BLOCK_ID] = f"ユーザグループ名は{OPTION_VALUE_LIMIT}文字以下にしてください。"

        if errors:
            await ack(response_action="errors", errors=errors)
            return

        restored_groups = await brain.restore("usergroups")
        groups = UserGroups.model_validate_json(restored_groups) if restored_groups else UserGroups.model_validate({})

        del groups[private_metadata.selected_group_name]

        if group.name in groups:
            await retryable_client.chat_postEphemeral(
                channel=private_metadata.channel_id,
                user=private_metadata.user_id,
                text=f"`{group.name}` は既に存在します。",
            )
            return

        groups[group.name] = group

        await brain.save(
            "usergroups",
            json.dumps(groups.root, ensure_ascii=False, default=lambda x: x.model_dump()).encode("utf-8"),
        )

        await retryable_client.chat_postEphemeral(
            channel=private_metadata.channel_id,
            user=private_metadata.user_id,
            text=f"`{group.name}` を編集しました。",
        )

        await ack()

    @bolt.view(DELETE_CONFIRM_CALLBACK_ID)
    async def handle_delete_confirm(
        ack: slack_bolt.context.ack.async_ack.AsyncAck,
        client: slack_sdk.web.async_client.AsyncWebClient,
        body: collections.abc.Mapping[str, typing.Any],
    ):
        await ack()

        retryable_client = slack.RetryableAsyncWebClient(client)

        private_metadata = PrivateMetadata.model_validate_json(body["view"]["private_metadata"])

        name = body["view"]["blocks"][0]["block_id"]

        restored_groups = await brain.restore("usergroups")
        groups = UserGroups.model_validate_json(restored_groups) if restored_groups else UserGroups.model_validate({})

        del groups[private_metadata.selected_group_name]

        await brain.save(
            "usergroups",
            json.dumps(groups.root, ensure_ascii=False, default=lambda x: x.model_dump()).encode("utf-8"),
        )

        await retryable_client.chat_postEphemeral(
            channel=private_metadata.channel_id,
            user=private_metadata.user_id,
            text=f"`{name}` を削除しました。",
        )

    @bolt.view(USE_CONFIRM_CALLBACK_ID)
    async def handle_use_confirm(
        ack: slack_bolt.context.ack.async_ack.AsyncAck,
        client: slack_sdk.web.async_client.AsyncWebClient,
        body: collections.abc.Mapping[str, typing.Any],
    ):
        await ack()

        retryable_client = slack.RetryableAsyncWebClient(client)

        private_metadata = PrivateMetadata.model_validate_json(body["view"]["private_metadata"])

        name = body["view"]["blocks"][0]["block_id"]

        restored_groups = await brain.restore("usergroups")
        groups = UserGroups.model_validate_json(restored_groups) if restored_groups else UserGroups.model_validate({})

        group = groups[name]

        if group.channel_id is not None and group.channel_id != private_metadata.channel_id:
            await retryable_client.chat_postEphemeral(
                channel=private_metadata.channel_id,
                user=private_metadata.user_id,
                text=f"`{group.name}` はこのチャンネルでは利用できません。",
            )
            return

        users_text = " ".join([f"<@{user}>" for user in group.users])
        await retryable_client.chat_postEphemeral(
            channel=private_metadata.channel_id,
            user=private_metadata.user_id,
            text=f"`{group.name}` のユーザグループにメンションします。",
        )
        await retryable_client.chat_postMessage(channel=private_metadata.channel_id, text=users_text)

    # Add

    async def add_group(
        ack: slack_bolt.context.ack.async_ack.AsyncAck,
        client: slack_sdk.web.async_client.AsyncWebClient,
        retryable_client: slack_sdk.web.async_client.AsyncWebClient,
        body: collections.abc.Mapping[str, typing.Any],
    ):
        await ack()

        private_metadata = PrivateMetadata(channel_id=body["channel_id"], user_id=body["user_id"])
        await retryable_client.views_open(
            trigger_id=body["trigger_id"],
            view={
                "type": "modal",
                "callback_id": ADD_CONFIRM_CALLBACK_ID,
                "private_metadata": json.dumps(private_metadata.model_dump(), ensure_ascii=False),
                "title": {"type": "plain_text", "text": "ユーザグループを追加する"},
                "blocks": [
                    {
                        "type": "input",
                        "block_id": NAME_BLOCK_ID,
                        "label": {"type": "plain_text", "text": "ユーザグループ名"},
                        "element": {
                            "type": "plain_text_input",
                            "action_id": NAME_ACTION_ID,
                            "placeholder": {"type": "plain_text", "text": "開発チーム"},
                        },
                    },
                    {
                        "type": "input",
                        "block_id": USERS_BLOCK_ID,
                        "label": {"type": "plain_text", "text": "ユーザ"},
                        "element": {
                            "type": "multi_conversations_select",
                            "action_id": USERS_ACTION_ID,
                            "placeholder": {
                                "type": "plain_text",
                                "text": "Select users",
                            },
                            "filter": {
                                "include": [
                                    "im",
                                ],
                                "exclude_bot_users": True,
                            },
                        },
                    },
                    {
                        "type": "input",
                        "block_id": LOCAL_BLOCK_ID,
                        "label": {"type": "plain_text", "text": "チャンネル限定"},
                        "element": {
                            "type": "radio_buttons",
                            "action_id": LOCAL_ACTION_ID,
                            "options": [
                                {
                                    "text": {
                                        "type": "plain_text",
                                        "text": "このチャンネル限定にする",
                                    },
                                    "value": Toggle.On,
                                },
                                {
                                    "text": {
                                        "type": "plain_text",
                                        "text": "全チャンネルで利用可能にする",
                                    },
                                    "value": Toggle.Off,
                                }
                            ],
                            "initial_option": {
                                "text": {
                                    "type": "plain_text",
                                    "text": "このチャンネル限定にする",
                                },
                                "value": Toggle.On,
                            },
                        },
                    },
                ],
                "close": {"type": "plain_text", "text": "キャンセル"},
                "submit": {"type": "plain_text", "text": "追加する"},
            },
        )

    # Edit

    async def edit_group(
        ack: slack_bolt.context.ack.async_ack.AsyncAck,
        client: slack_sdk.web.async_client.AsyncWebClient,
        retryable_client: slack_sdk.web.async_client.AsyncWebClient,
        body: collections.abc.Mapping[str, typing.Any],
    ):
        await ack()

        restored_groups = await brain.restore("usergroups")
        groups = UserGroups.model_validate_json(restored_groups) if restored_groups else UserGroups.model_validate({})

        private_metadata = PrivateMetadata(channel_id=body["channel_id"], user_id=body["user_id"])

        filtered_groups = {
            name: group for name, group in groups.items()
            if group.channel_id is None or group.channel_id == body["channel_id"]
        }

        if not filtered_groups:
            await retryable_client.chat_postEphemeral(
                channel=private_metadata.channel_id,
                user=private_metadata.user_id,
                text="このチャンネルで編集できるユーザグループがありません。",
            )
            return

        await retryable_client.views_open(
            trigger_id=body["trigger_id"],
            view={
                "type": "modal",
                "callback_id": EDIT_GROUP_CALLBACK_ID,
                "private_metadata": json.dumps(private_metadata.model_dump(), ensure_ascii=False),
                "title": {"type": "plain_text", "text": "ユーザグループを編集する"},
                "blocks": [
                    {
                        "type": "input",
                        "block_id": EDIT_GROUP_BLOCK_ID,
                        "label": {"type": "plain_text", "text": "ユーザグループ名"},
                        "element": {
                            "type": "external_select",
                            "action_id": EDIT_GROUP_ACTION_ID,
                            "placeholder": {"type": "plain_text", "text": "ユーザグループ名を入力してください"},
                            "min_query_length": 0,
                        },
                    },
                ],
                "close": {"type": "plain_text", "text": "キャンセル"},
                "submit": {"type": "plain_text", "text": ":warning: 編集する", "emoji": True},
            },
        )

    @bolt.options(EDIT_GROUP_ACTION_ID)
    async def handle_edit_group_options(
        ack: slack_bolt.context.ack.async_ack.AsyncAck,
        body: collections.abc.Mapping[str, typing.Any],
    ):
        query = body.get("value", "").lower()

        restored_groups = await brain.restore("usergroups")
        groups = UserGroups.model_validate_json(restored_groups) if restored_groups else UserGroups.model_validate({})

        channel_id = body["view"]["private_metadata"]
        private_metadata = PrivateMetadata.model_validate_json(channel_id)

        filtered_groups = {
            name: group for name, group in groups.items()
            if group.channel_id is None or group.channel_id == private_metadata.channel_id
        }

        options = []
        for name, group in filtered_groups.items():
            if query in name.lower():
                option_text = name[:OPTION_TEXT_LIMIT - 3] + "..." if len(name) > OPTION_TEXT_LIMIT else name
                options.append({
                    "text": {"type": "plain_text", "text": option_text},
                    "value": name,
                })

        await ack(options=options[:100])

    @bolt.view(EDIT_GROUP_CALLBACK_ID)
    async def handle_edit_group(
        ack: slack_bolt.context.ack.async_ack.AsyncAck,
        client: slack_sdk.web.async_client.AsyncWebClient,
        body: collections.abc.Mapping[str, typing.Any],
    ):
        await ack()

        retryable_client = slack.RetryableAsyncWebClient(client)

        private_metadata = PrivateMetadata.model_validate_json(body["view"]["private_metadata"])

        values = body["view"]["state"]["values"]
        name = values[EDIT_GROUP_BLOCK_ID][EDIT_GROUP_ACTION_ID]["selected_option"]["value"]

        restored_groups = await brain.restore("usergroups")
        groups = UserGroups.model_validate_json(restored_groups) if restored_groups else UserGroups.model_validate({})

        group = groups[name]
        private_metadata.selected_group_name = group.name

        await retryable_client.views_open(
            trigger_id=body["trigger_id"],
            view={
                "type": "modal",
                "callback_id": EDIT_CONFIRM_CALLBACK_ID,
                "private_metadata": json.dumps(private_metadata.model_dump(), ensure_ascii=False),
                "title": {"type": "plain_text", "text": "ユーザグループを編集する"},
                "blocks": [
                    {
                        "type": "input",
                        "block_id": NAME_BLOCK_ID,
                        "label": {"type": "plain_text", "text": "ユーザグループ名"},
                        "element": {
                            "type": "plain_text_input",
                            "action_id": NAME_ACTION_ID,
                            "initial_value": group.name,
                        },
                    },
                    {
                        "type": "input",
                        "block_id": USERS_BLOCK_ID,
                        "label": {"type": "plain_text", "text": "ユーザ"},
                        "element": {
                            "type": "multi_conversations_select",
                            "action_id": USERS_ACTION_ID,
                            "initial_conversations": group.users,
                            "placeholder": {
                                "type": "plain_text",
                                "text": "Select users",
                            },
                            "filter": {
                                "include": [
                                    "im",
                                ],
                                "exclude_bot_users": True,
                            },
                        },
                    },
                    {
                        "type": "input",
                        "block_id": LOCAL_BLOCK_ID,
                        "label": {"type": "plain_text", "text": "チャンネル限定"},
                        "element": {
                            "type": "radio_buttons",
                            "action_id": LOCAL_ACTION_ID,
                            "options": [
                                {
                                    "text": {
                                        "type": "plain_text",
                                        "text": "このチャンネル限定にする",
                                    },
                                    "value": Toggle.On,
                                },
                                {
                                    "text": {
                                        "type": "plain_text",
                                        "text": "全チャンネルで利用可能にする",
                                    },
                                    "value": Toggle.Off,
                                }
                            ],
                            "initial_option": {
                                "text": {
                                    "type": "plain_text",
                                    "text": "このチャンネル限定にする" if group.channel_id is not None else "全チャンネルで利用可能にする",
                                },
                                "value": Toggle.On if group.channel_id is not None else Toggle.Off,
                            },
                        },
                    },
                ],
                "close": {"type": "plain_text", "text": "キャンセル"},
                "submit": {"type": "plain_text", "text": ":warning: 編集する", "emoji": True},
            },
        )

    # Delete

    async def delete_group(
        ack: slack_bolt.context.ack.async_ack.AsyncAck,
        client: slack_sdk.web.async_client.AsyncWebClient,
        retryable_client: slack_sdk.web.async_client.AsyncWebClient,
        body: collections.abc.Mapping[str, typing.Any],
    ):
        await ack()

        restored_groups = await brain.restore("usergroups")
        groups = UserGroups.model_validate_json(restored_groups) if restored_groups else UserGroups.model_validate({})

        private_metadata = PrivateMetadata(channel_id=body["channel_id"], user_id=body["user_id"])

        filtered_groups = {
            name: group for name, group in groups.items()
            if group.channel_id is None or group.channel_id == body["channel_id"]
        }

        if len(filtered_groups) == 0:
            await retryable_client.chat_postEphemeral(
                channel=private_metadata.channel_id,
                user=private_metadata.user_id,
                text="削除するユーザグループがありません。",
            )
            return

        await retryable_client.views_open(
            trigger_id=body["trigger_id"],
            view={
                "type": "modal",
                "callback_id": DELETE_GROUP_CALLBACK_ID,
                "private_metadata": json.dumps(private_metadata.model_dump(), ensure_ascii=False),
                "title": {"type": "plain_text", "text": "ユーザグループを削除する"},
                "blocks": [
                    {
                        "type": "input",
                        "block_id": DELETE_GROUP_BLOCK_ID,
                        "label": {"type": "plain_text", "text": "ユーザグループ名"},
                        "element": {
                            "type": "external_select",
                            "action_id": DELETE_GROUP_ACTION_ID,
                            "placeholder": {"type": "plain_text", "text": "ユーザグループ名を入力してください"},
                            "min_query_length": 0,
                        },
                    },
                ],
                "close": {"type": "plain_text", "text": "キャンセル"},
                "submit": {"type": "plain_text", "text": ":warning: 削除する", "emoji": True},
            },
        )

    @bolt.options(DELETE_GROUP_ACTION_ID)
    async def handle_delete_group_options(
        ack: slack_bolt.context.ack.async_ack.AsyncAck,
        body: collections.abc.Mapping[str, typing.Any],
    ):
        query = body.get("value", "").lower()

        restored_groups = await brain.restore("usergroups")
        groups = UserGroups.model_validate_json(restored_groups) if restored_groups else UserGroups.model_validate({})

        channel_id = body["view"]["private_metadata"]
        private_metadata = PrivateMetadata.model_validate_json(channel_id)

        filtered_groups = {
            name: group for name, group in groups.items()
            if group.channel_id is None or group.channel_id == private_metadata.channel_id
        }

        options = []
        for name, group in filtered_groups.items():
            if query in name.lower():
                option_text = name[:OPTION_TEXT_LIMIT - 3] + "..." if len(name) > OPTION_TEXT_LIMIT else name
                options.append({
                    "text": {"type": "plain_text", "text": option_text},
                    "value": name,
                })

        await ack(options=options[:100])

    @bolt.view(DELETE_GROUP_CALLBACK_ID)
    async def handle_delete_group(
        ack: slack_bolt.context.ack.async_ack.AsyncAck,
        client: slack_sdk.web.async_client.AsyncWebClient,
        body: collections.abc.Mapping[str, typing.Any],
    ):
        await ack()

        retryable_client = slack.RetryableAsyncWebClient(client)

        private_metadata = PrivateMetadata.model_validate_json(body["view"]["private_metadata"])

        values = body["view"]["state"]["values"]
        name = values[DELETE_GROUP_BLOCK_ID][DELETE_GROUP_ACTION_ID]["selected_option"]["value"]

        restored_groups = await brain.restore("usergroups")
        groups = UserGroups.model_validate_json(restored_groups) if restored_groups else UserGroups.model_validate({})

        group = groups[name]
        private_metadata.selected_group_name = group.name

        await retryable_client.views_open(
            trigger_id=body["trigger_id"],
            view={
                "type": "modal",
                "callback_id": DELETE_CONFIRM_CALLBACK_ID,
                "private_metadata": json.dumps(private_metadata.model_dump(), ensure_ascii=False),
                "title": {"type": "plain_text", "text": "これを削除しますか？"},
                "blocks": [
                    {
                        "type": "section",
                        "block_id": group.name,
                        "text": {
                            "type": "mrkdwn",
                            "text": format_markdown(group),
                        },
                    },
                ],
                "close": {"type": "plain_text", "text": "キャンセル"},
                "submit": {"type": "plain_text", "text": ":warning: 確認する"},
            },
        )

    # List

    async def list_group(
        ack: slack_bolt.context.ack.async_ack.AsyncAck,
        client: slack_sdk.web.async_client.AsyncWebClient,
        retryable_client: slack_sdk.web.async_client.AsyncWebClient,
        body: collections.abc.Mapping[str, typing.Any],
        page: int = 0,
    ):
        await ack()

        restored_groups = await brain.restore("usergroups")
        groups = UserGroups.model_validate_json(restored_groups) if restored_groups else UserGroups.model_validate({})

        private_metadata = PrivateMetadata(channel_id=body["channel_id"], user_id=body["user_id"], list_page=page)

        filtered_groups = {
            name: group for name, group in groups.items()
            if group.channel_id is None or group.channel_id == body["channel_id"]
        }

        if not filtered_groups:
            await retryable_client.chat_postEphemeral(
                channel=private_metadata.channel_id,
                user=private_metadata.user_id,
                text="このチャンネルで利用できるユーザグループがありません。",
            )
            return

        groups_list = list(filtered_groups.items())
        total_groups = len(groups_list)
        total_pages = (total_groups + ITEMS_PER_PAGE - 1) // ITEMS_PER_PAGE

        start_idx = page * ITEMS_PER_PAGE
        end_idx = min(start_idx + ITEMS_PER_PAGE, total_groups)
        page_groups = groups_list[start_idx:end_idx]

        blocks = []

        blocks.append({
            "type": "section",
            "text": {
                "type": "mrkdwn",
                "text": f"*ページ {page + 1} / {total_pages}* (全 {total_groups} グループ)",
            },
        })
        blocks.append({"type": "divider"})

        for name, group in page_groups:
            blocks.append(
                {
                    "type": "section",
                    "block_id": group.name,
                    "text": {
                        "type": "mrkdwn",
                        "text": format_markdown(group),
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
                                "text": "これにメンションする",
                            },
                            "value": group.name,
                            "action_id": LIST_GROUP_ACTION_ID,
                        },
                    ],
                },
            )

        if total_pages > 1:
            blocks.append({"type": "divider"})
            nav_elements = []

            if page > 0:
                nav_elements.append({
                    "type": "button",
                    "text": {
                        "type": "plain_text",
                        "text": "← 前のページ",
                    },
                    "action_id": LIST_GROUP_PREV_ACTION_ID,
                })

            if page < total_pages - 1:
                nav_elements.append({
                    "type": "button",
                    "text": {
                        "type": "plain_text",
                        "text": "次のページ →",
                    },
                    "action_id": LIST_GROUP_NEXT_ACTION_ID,
                })

            if nav_elements:
                blocks.append({
                    "type": "actions",
                    "elements": nav_elements,
                })

        await retryable_client.views_open(
            trigger_id=body["trigger_id"],
            view={
                "type": "modal",
                "callback_id": LIST_GROUP_CALLBACK_ID,
                "private_metadata": json.dumps(private_metadata.model_dump(), ensure_ascii=False),
                "title": {"type": "plain_text", "text": "ユーザグループにメンションする"},
                "blocks": blocks,
                "close": {"type": "plain_text", "text": "キャンセル"},
            },
        )

    @bolt.action(LIST_GROUP_ACTION_ID)
    async def handle_list_group_button(
        ack: slack_bolt.context.ack.async_ack.AsyncAck,
        client: slack_sdk.web.async_client.AsyncWebClient,
        body: collections.abc.Mapping[str, typing.Any],
    ):
        await ack()

        retryable_client = slack.RetryableAsyncWebClient(client)

        name = body["actions"][0]["value"]

        restored_groups = await brain.restore("usergroups")
        groups = UserGroups.model_validate_json(restored_groups) if restored_groups else UserGroups.model_validate({})

        group = groups[name]

        await retryable_client.views_update(
            view_id=body["container"]["view_id"],
            view={
                "type": "modal",
                "callback_id": USE_CONFIRM_CALLBACK_ID,
                "private_metadata": body["view"]["private_metadata"],
                "title": {"type": "plain_text", "text": "これにメンションしますか？"},
                "blocks": [
                    {
                        "type": "section",
                        "block_id": group.name,
                        "text": {
                            "type": "mrkdwn",
                            "text": format_markdown(group),
                        },
                    },
                ],
                "close": {"type": "plain_text", "text": "キャンセル"},
                "submit": {"type": "plain_text", "text": "確認する"},
            },
        )

    @bolt.action(LIST_GROUP_PREV_ACTION_ID)
    async def handle_list_group_prev(
        ack: slack_bolt.context.ack.async_ack.AsyncAck,
        client: slack_sdk.web.async_client.AsyncWebClient,
        body: collections.abc.Mapping[str, typing.Any],
    ):
        await ack()

        retryable_client = slack.RetryableAsyncWebClient(client)

        private_metadata = PrivateMetadata.model_validate_json(body["view"]["private_metadata"])
        new_page = max(0, private_metadata.list_page - 1)

        restored_groups = await brain.restore("usergroups")
        groups = UserGroups.model_validate_json(restored_groups) if restored_groups else UserGroups.model_validate({})

        private_metadata.list_page = new_page

        filtered_groups = {
            name: group for name, group in groups.items()
            if group.channel_id is None or group.channel_id == private_metadata.channel_id
        }

        groups_list = list(filtered_groups.items())
        total_groups = len(groups_list)
        total_pages = (total_groups + ITEMS_PER_PAGE - 1) // ITEMS_PER_PAGE

        start_idx = new_page * ITEMS_PER_PAGE
        end_idx = min(start_idx + ITEMS_PER_PAGE, total_groups)
        page_groups = groups_list[start_idx:end_idx]

        blocks = []

        blocks.append({
            "type": "section",
            "text": {
                "type": "mrkdwn",
                "text": f"*ページ {new_page + 1} / {total_pages}* (全 {total_groups} グループ)",
            },
        })
        blocks.append({"type": "divider"})

        for name, group in page_groups:
            blocks.append(
                {
                    "type": "section",
                    "block_id": group.name,
                    "text": {
                        "type": "mrkdwn",
                        "text": format_markdown(group),
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
                                "text": "これにメンションする",
                            },
                            "value": group.name,
                            "action_id": LIST_GROUP_ACTION_ID,
                        },
                    ],
                },
            )

        if total_pages > 1:
            blocks.append({"type": "divider"})
            nav_elements = []

            if new_page > 0:
                nav_elements.append({
                    "type": "button",
                    "text": {
                        "type": "plain_text",
                        "text": "← 前のページ",
                    },
                    "action_id": LIST_GROUP_PREV_ACTION_ID,
                })

            if new_page < total_pages - 1:
                nav_elements.append({
                    "type": "button",
                    "text": {
                        "type": "plain_text",
                        "text": "次のページ →",
                    },
                    "action_id": LIST_GROUP_NEXT_ACTION_ID,
                })

            if nav_elements:
                blocks.append({
                    "type": "actions",
                    "elements": nav_elements,
                })

        await retryable_client.views_update(
            view_id=body["container"]["view_id"],
            view={
                "type": "modal",
                "callback_id": LIST_GROUP_CALLBACK_ID,
                "private_metadata": json.dumps(private_metadata.model_dump(), ensure_ascii=False),
                "title": {"type": "plain_text", "text": "ユーザグループにメンションする"},
                "blocks": blocks,
                "close": {"type": "plain_text", "text": "キャンセル"},
            },
        )

    @bolt.action(LIST_GROUP_NEXT_ACTION_ID)
    async def handle_list_group_next(
        ack: slack_bolt.context.ack.async_ack.AsyncAck,
        client: slack_sdk.web.async_client.AsyncWebClient,
        body: collections.abc.Mapping[str, typing.Any],
    ):
        await ack()

        retryable_client = slack.RetryableAsyncWebClient(client)

        private_metadata = PrivateMetadata.model_validate_json(body["view"]["private_metadata"])

        restored_groups = await brain.restore("usergroups")
        groups = UserGroups.model_validate_json(restored_groups) if restored_groups else UserGroups.model_validate({})

        filtered_groups = {
            name: group for name, group in groups.items()
            if group.channel_id is None or group.channel_id == private_metadata.channel_id
        }

        total_groups = len(filtered_groups)
        total_pages = (total_groups + ITEMS_PER_PAGE - 1) // ITEMS_PER_PAGE
        new_page = min(total_pages - 1, private_metadata.list_page + 1)

        private_metadata.list_page = new_page

        groups_list = list(filtered_groups.items())

        start_idx = new_page * ITEMS_PER_PAGE
        end_idx = min(start_idx + ITEMS_PER_PAGE, total_groups)
        page_groups = groups_list[start_idx:end_idx]

        blocks = []

        blocks.append({
            "type": "section",
            "text": {
                "type": "mrkdwn",
                "text": f"*ページ {new_page + 1} / {total_pages}* (全 {total_groups} グループ)",
            },
        })
        blocks.append({"type": "divider"})

        for name, group in page_groups:
            blocks.append(
                {
                    "type": "section",
                    "block_id": group.name,
                    "text": {
                        "type": "mrkdwn",
                        "text": format_markdown(group),
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
                                "text": "これにメンションする",
                            },
                            "value": group.name,
                            "action_id": LIST_GROUP_ACTION_ID,
                        },
                    ],
                },
            )

        if total_pages > 1:
            blocks.append({"type": "divider"})
            nav_elements = []

            if new_page > 0:
                nav_elements.append({
                    "type": "button",
                    "text": {
                        "type": "plain_text",
                        "text": "← 前のページ",
                    },
                    "action_id": LIST_GROUP_PREV_ACTION_ID,
                })

            if new_page < total_pages - 1:
                nav_elements.append({
                    "type": "button",
                    "text": {
                        "type": "plain_text",
                        "text": "次のページ →",
                    },
                    "action_id": LIST_GROUP_NEXT_ACTION_ID,
                })

            if nav_elements:
                blocks.append({
                    "type": "actions",
                    "elements": nav_elements,
                })

        await retryable_client.views_update(
            view_id=body["container"]["view_id"],
            view={
                "type": "modal",
                "callback_id": LIST_GROUP_CALLBACK_ID,
                "private_metadata": json.dumps(private_metadata.model_dump(), ensure_ascii=False),
                "title": {"type": "plain_text", "text": "ユーザグループにメンションする"},
                "blocks": blocks,
                "close": {"type": "plain_text", "text": "キャンセル"},
            },
        )

    # Select

    async def select_group(
        ack: slack_bolt.context.ack.async_ack.AsyncAck,
        client: slack_sdk.web.async_client.AsyncWebClient,
        retryable_client: slack_sdk.web.async_client.AsyncWebClient,
        body: collections.abc.Mapping[str, typing.Any],
    ):
        await ack()

        restored_groups = await brain.restore("usergroups")
        groups = UserGroups.model_validate_json(restored_groups) if restored_groups else UserGroups.model_validate({})

        private_metadata = PrivateMetadata(channel_id=body["channel_id"], user_id=body["user_id"])

        filtered_groups = {
            name: group for name, group in groups.items()
            if group.channel_id is None or group.channel_id == body["channel_id"]
        }

        if not filtered_groups:
            await retryable_client.chat_postEphemeral(
                channel=private_metadata.channel_id,
                user=private_metadata.user_id,
                text="このチャンネルで利用できるユーザグループがありません。",
            )
            return

        await retryable_client.views_open(
            trigger_id=body["trigger_id"],
            view={
                "type": "modal",
                "callback_id": SELECT_GROUP_CALLBACK_ID,
                "private_metadata": json.dumps(private_metadata.model_dump(), ensure_ascii=False),
                "title": {"type": "plain_text", "text": "ユーザグループを利用する"},
                "blocks": [
                    {
                        "type": "input",
                        "block_id": SELECT_GROUP_BLOCK_ID,
                        "label": {"type": "plain_text", "text": "ユーザグループ名"},
                        "element": {
                            "type": "external_select",
                            "action_id": SELECT_GROUP_ACTION_ID,
                            "placeholder": {"type": "plain_text", "text": "ユーザグループ名を入力してください"},
                            "min_query_length": 0,
                        },
                    },
                ],
                "close": {"type": "plain_text", "text": "キャンセル"},
                "submit": {"type": "plain_text", "text": "利用する"},
            },
        )

    @bolt.options(SELECT_GROUP_ACTION_ID)
    async def handle_select_group_options(
        ack: slack_bolt.context.ack.async_ack.AsyncAck,
        body: collections.abc.Mapping[str, typing.Any],
    ):
        query = body.get("value", "").lower()

        restored_groups = await brain.restore("usergroups")
        groups = UserGroups.model_validate_json(restored_groups) if restored_groups else UserGroups.model_validate({})

        channel_id = body["view"]["private_metadata"]
        private_metadata = PrivateMetadata.model_validate_json(channel_id)

        filtered_groups = {
            name: group for name, group in groups.items()
            if group.channel_id is None or group.channel_id == private_metadata.channel_id
        }

        options = []
        for name, group in filtered_groups.items():
            if query in name.lower():
                option_text = name[:OPTION_TEXT_LIMIT - 3] + "..." if len(name) > OPTION_TEXT_LIMIT else name
                options.append({
                    "text": {"type": "plain_text", "text": option_text},
                    "value": name,
                })

        await ack(options=options[:100])

    @bolt.view(SELECT_GROUP_CALLBACK_ID)
    async def handle_select_group(
        ack: slack_bolt.context.ack.async_ack.AsyncAck,
        client: slack_sdk.web.async_client.AsyncWebClient,
        body: collections.abc.Mapping[str, typing.Any],
    ):
        await ack()

        retryable_client = slack.RetryableAsyncWebClient(client)

        values = body["view"]["state"]["values"]
        name = values[SELECT_GROUP_BLOCK_ID][SELECT_GROUP_ACTION_ID]["selected_option"]["value"]

        restored_groups = await brain.restore("usergroups")
        groups = UserGroups.model_validate_json(restored_groups) if restored_groups else UserGroups.model_validate({})

        group = groups[name]

        await retryable_client.views_open(
            trigger_id=body["trigger_id"],
            view={
                "type": "modal",
                "callback_id": USE_CONFIRM_CALLBACK_ID,
                "private_metadata": body["view"]["private_metadata"],
                "title": {"type": "plain_text", "text": "これにメンションしますか？"},
                "blocks": [
                    {
                        "type": "section",
                        "block_id": group.name,
                        "text": {
                            "type": "mrkdwn",
                            "text": format_markdown(group),
                        },
                    },
                ],
                "close": {"type": "plain_text", "text": "キャンセル"},
                "submit": {"type": "plain_text", "text": "利用する"},
            },
        )

    @bolt.command("/ug")
    @bolt.command("/_ug")
    async def ug(
        ack: slack_bolt.context.ack.async_ack.AsyncAck,
        client: slack_sdk.web.async_client.AsyncWebClient,
        body: collections.abc.Mapping[str, typing.Any],
    ):
        await ack()

        retryable_client = slack.RetryableAsyncWebClient(client)

        match body["text"]:
            case "add":
                await add_group(ack, client, retryable_client, body)
            case "edit":
                await edit_group(ack, client, retryable_client, body)
            case "delete":
                await delete_group(ack, client, retryable_client, body)
            case "list":
                await list_group(ack, client, retryable_client, body)
            case "select":
                await select_group(ack, client, retryable_client, body)
            case _:
                await retryable_client.chat_postEphemeral(
                    channel=body["channel_id"],
                    user=body["user_id"],
                    text="`/usergroup list`, `/usergroup select` または  `/usergroup add`, `/usergroup edit`, `/usergroup delete` と入力してください。",
                )
