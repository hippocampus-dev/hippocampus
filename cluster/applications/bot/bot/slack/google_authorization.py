import asyncio
import collections.abc
import secrets
import time
import typing

import aiohttp
import aiohttp.client_exceptions
import google.oauth2.credentials
import redis.asyncio
import slack_bolt.async_app
import slack_bolt.context.ack.async_ack

import bot.slack
import bot.slack.i18n
import bot.telemetry
import cortex.exceptions

# https://www.rfc-editor.org/rfc/rfc8628#section-3.4
_DEVICE_CODE_GRANT_TYPE = "urn:ietf:params:oauth:grant-type:device_code"

_ALLOW_ACTION_ID = "google-authorization-allow"
_DENY_ACTION_ID = "google-authorization-deny"

_redis_client: redis.asyncio.Redis | None = None


def _denied_key(authorization_id: str) -> str:
    return f"google-authorization-denied:{authorization_id}"


async def register(
    bolt: slack_bolt.async_app.AsyncApp,
    redis_client: redis.asyncio.Redis,
    timeout_seconds: int,
):
    global _redis_client
    _redis_client = redis_client

    @bolt.action(_ALLOW_ACTION_ID)
    async def handle_allow(ack: slack_bolt.context.ack.async_ack.AsyncAck):
        # The button carries a url, so Slack opens the consent page without the app doing anything
        await ack()

    @bolt.action(_DENY_ACTION_ID)
    async def handle_deny(
        ack: slack_bolt.context.ack.async_ack.AsyncAck,
        body: collections.abc.Mapping[str, typing.Any],
    ):
        await ack()

        # The request that is waiting may run in another pod
        await redis_client.set(
            _denied_key(body["actions"][0]["value"]), 1, ex=timeout_seconds
        )


class UserGoogleCredentialsProvider:
    oauth_bridge_url: str
    scopes: collections.abc.Sequence[str]
    timeout_seconds: int
    shared_credentials: google.oauth2.credentials.Credentials
    attempted: bool
    credentials: google.oauth2.credentials.Credentials | None

    def __init__(
        self,
        oauth_bridge_url: str,
        scopes: collections.abc.Sequence[str],
        timeout_seconds: int,
        shared_credentials: google.oauth2.credentials.Credentials,
    ):
        self.oauth_bridge_url = oauth_bridge_url
        self.scopes = scopes
        self.timeout_seconds = timeout_seconds
        self.shared_credentials = shared_credentials
        self.attempted = False
        self.credentials = None

    async def __call__(
        self, context: bot.slack.SlackContext, escalate: bool
    ) -> google.oauth2.credentials.Credentials | None:
        if not escalate:
            return self.shared_credentials

        # One authorization covers every Google URL the same request opens
        if self.attempted:
            return self.credentials
        self.attempted = True

        authorized: google.oauth2.credentials.Credentials | None = None
        # The device code must not reach the browser, so the button carries this instead
        authorization_id = secrets.token_urlsafe(32)
        timeout = aiohttp.ClientTimeout(total=30)
        async with aiohttp.ClientSession(timeout=timeout, trust_env=True) as session:
            try:
                async with session.post(
                    f"{self.oauth_bridge_url}/device/code",
                    data={"scope": " ".join(self.scopes)},
                ) as response:
                    if response.status != 200:
                        e = ValueError(f"invalid response status: {response.status}")
                        match response.status:
                            case 409 | 429 | 502 | 503 | 504:
                                raise cortex.exceptions.RetryableError(e) from e
                        raise e
                    device_code_response = await response.json()

                device_code = device_code_response["device_code"]
                interval = device_code_response["interval"]

                await context.client.chat_postEphemeral(  # Tier 4 (100+ per minute)
                    channel=context.event["channel"],
                    thread_ts=context.event.get("thread_ts") or context.event["ts"],
                    user=context.user["id"],
                    text=bot.slack.i18n.translate(
                        "The shared account cannot read this document.",
                        locale=context.locale,
                    ),
                    blocks=[
                        {
                            "type": "section",
                            "text": {
                                "type": "mrkdwn",
                                "text": bot.slack.i18n.translate(
                                    "The shared account cannot read this document.",
                                    locale=context.locale,
                                ),
                            },
                        },
                        {
                            "type": "actions",
                            "elements": [
                                {
                                    "type": "button",
                                    "style": "primary",
                                    "text": {
                                        "type": "plain_text",
                                        "text": bot.slack.i18n.translate(
                                            "Allow with my own permission",
                                            locale=context.locale,
                                        ),
                                    },
                                    "url": device_code_response["verification_uri"],
                                    "action_id": _ALLOW_ACTION_ID,
                                },
                                {
                                    "type": "button",
                                    "text": {
                                        "type": "plain_text",
                                        "text": bot.slack.i18n.translate(
                                            "Skip this document",
                                            locale=context.locale,
                                        ),
                                    },
                                    "action_id": _DENY_ACTION_ID,
                                    "value": authorization_id,
                                },
                            ],
                        },
                    ],
                )

                deadline = time.monotonic() + self.timeout_seconds
                while time.monotonic() < deadline:
                    await asyncio.sleep(interval)

                    if await _redis_client.exists(_denied_key(authorization_id)):
                        break

                    try:
                        async with session.post(
                            f"{self.oauth_bridge_url}/token",
                            data={
                                "grant_type": _DEVICE_CODE_GRANT_TYPE,
                                "device_code": device_code,
                            },
                        ) as response:
                            if response.status == 200:
                                token_response = await response.json()
                                authorized = google.oauth2.credentials.Credentials(
                                    token=token_response["access_token"],
                                )
                                break

                            # Every outcome the flow defines is reported as 400 with a JSON body
                            if response.status != 400:
                                bot.telemetry.logger.error(
                                    ValueError(
                                        f"invalid response status: {response.status}"
                                    )
                                )
                                break

                            token_response = await response.json()
                            match token_response.get("error"):
                                case "authorization_pending":
                                    continue
                                case "slow_down":
                                    interval = token_response["interval"]
                                    continue
                                case "expired_token":
                                    break
                            bot.telemetry.logger.error(
                                ValueError(
                                    f"authorization failed: {token_response.get('error')}"
                                )
                            )
                            break
                    except (
                        aiohttp.ClientConnectionError,  # ECONNREFUSED, EPIPE, ECONNRESET
                        aiohttp.client_exceptions.ServerDisconnectedError,
                        asyncio.TimeoutError,
                    ) as e:
                        # Retrying the whole event here would issue a second link while the user is on the first
                        bot.telemetry.logger.warning(e)
            except (
                aiohttp.ClientConnectionError,  # ECONNREFUSED, EPIPE, ECONNRESET
                aiohttp.client_exceptions.ServerDisconnectedError,
                asyncio.TimeoutError,
            ) as e:
                raise cortex.exceptions.RetryableError(e) from e
            except ValueError as e:
                bot.telemetry.logger.error(e)

        await _redis_client.delete(_denied_key(authorization_id))

        self.credentials = authorized
        return self.credentials
