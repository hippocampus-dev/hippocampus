#!/usr/bin/env sh

set -e

SOCKET_FILE=/var/tmp/mattermost_local.socket
WEBHOOK_ID_FILE=/mattermost/data/webhookId
TEAM=mattermost
CHANNEL=alert
USERNAME=alertmanager

# Local mode talks to the server over a unix socket and bypasses authentication,
# so no administrator credentials have to be stored anywhere.
mmctl() {
  /mattermost/bin/mmctl --local --config-path /tmp "$@"
}

ensure_webhook() {
  mmctl user create --username "$USERNAME" --email "${USERNAME}@localhost" --password "$(head -c 32 /dev/urandom | base64)" --email_verified --disable-welcome-email > /dev/null 2>&1 || true
  mmctl team create --name "$TEAM" --display_name "$TEAM" > /dev/null 2>&1 || true
  mmctl team users add "$TEAM" "$USERNAME" > /dev/null 2>&1 || true
  mmctl channel create --team "$TEAM" --name "$CHANNEL" --display_name "$CHANNEL" > /dev/null 2>&1 || true

  # sh has no pipefail, so each command is captured before it is parsed.
  # The webhook outlives the volume holding WEBHOOK_ID_FILE, so reuse it instead of creating a duplicate.
  webhooks=$(mmctl webhook list "$TEAM") || return 1
  webhook_id=$(echo "$webhooks" | sed -n "s/^Incoming:[[:space:]]*${USERNAME} (\([a-z0-9]*\).*/\1/p" | awk 'NR==1')
  if [ -n "$webhook_id" ]; then
    return 0
  fi

  users=$(mmctl user search "$USERNAME") || return 1
  channels=$(mmctl channel search --team "$TEAM" "$CHANNEL") || return 1
  user_id=$(echo "$users" | awk '/^id:/ {print $2; exit}')
  channel_id=$(echo "$channels" | sed -n 's/.*Channel ID :\([a-z0-9]*\).*/\1/p')

  if [ -z "$user_id" ] || [ -z "$channel_id" ]; then
    return 1
  fi

  # mmctl omits user_id from the request body in local mode, so the API is called directly.
  created=$(curl -fsS --retry 5 --retry-all-errors --unix-socket "$SOCKET_FILE" -X POST http://localhost/api/v4/hooks/incoming -H 'Content-Type: application/json' -d "{\"channel_id\":\"${channel_id}\",\"user_id\":\"${user_id}\",\"display_name\":\"${USERNAME}\"}") || return 1
  webhook_id=$(echo "$created" | sed -n 's/.*"id"[[:space:]]*:[[:space:]]*"\([a-z0-9]*\)".*/\1/p')

  [ -n "$webhook_id" ]
}

until [ -S "$SOCKET_FILE" ]; do
  echo "waiting for ${SOCKET_FILE}" >&2
  sleep 5
done

if [ -s "$WEBHOOK_ID_FILE" ] && mmctl webhook show "$(cat "$WEBHOOK_ID_FILE")" > /dev/null 2>&1; then
  exit 0
fi

# Exiting non-zero here would make kubelet kill an otherwise healthy server, so keep retrying.
until ensure_webhook; do
  echo "retrying webhook bootstrap" >&2
  sleep 5
done

echo "$webhook_id" > "$WEBHOOK_ID_FILE"
