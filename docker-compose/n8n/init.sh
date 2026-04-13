#!/usr/bin/env bash

set -eo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

N8N_BASE="http://n8n:5678"

if curl -fsSL "${N8N_BASE}/rest/settings" | jq -re '.data.userManagement.showSetupOnFirstLoad == true' > /dev/null; then
  jq -n --arg email "$N8N_OWNER_EMAIL" --arg firstName "$N8N_OWNER_FIRST_NAME" --arg lastName "$N8N_OWNER_LAST_NAME" --arg password "$N8N_OWNER_PASSWORD" '{
    email: $email,
    firstName: $firstName,
    lastName: $lastName,
    password: $password
  }' | curl -fsSL -XPOST -H "Content-Type: application/json" "${N8N_BASE}/rest/owner/setup" -d @- > /dev/null
fi

token=$(jq -n --arg email "$N8N_OWNER_EMAIL" --arg password "$N8N_OWNER_PASSWORD" '{
  emailOrLdapLoginId: $email,
  password: $password
}' | curl -fsSLo /dev/null -D - -XPOST -H "Content-Type: application/json" "${N8N_BASE}/rest/login" -d @- | tr -d '\r' | sed -n 's/.*n8n-auth=\([^;]*\).*/\1/p')

if [ -z "$token" ]; then
  echo "failed to obtain auth token" >&2
  exit 1
fi

existing_credentials=$(curl -fsSL -H "Cookie: n8n-auth=${token}" "${N8N_BASE}/rest/credentials" | jq -re '[.data[].name]' || echo '[]')

if [ -n "$N8N_SLACK_BOT_TOKEN" ]; then
  credential_name="Slack Bot Token"

  if echo "$existing_credentials" | jq -re --arg name "$credential_name" 'index($name) != null' > /dev/null; then
    echo "credential already exists: $credential_name"
  else
    jq -n --arg name "$credential_name" --arg accessToken "$N8N_SLACK_BOT_TOKEN" '{
      name: $name,
      type: "slackApi",
      data: {
        accessToken: $accessToken
      }
    }' | curl -fsSL -H "Cookie: n8n-auth=${token}" -H "Content-Type: application/json" -XPOST "${N8N_BASE}/rest/credentials" -d @- > /dev/null
    echo "created credential: $credential_name"
  fi
fi

credentials=$(curl -fsSL -H "Cookie: n8n-auth=${token}" "${N8N_BASE}/rest/credentials" || echo '{"data":[]}')
slack_credential_id=$(echo "$credentials" | jq -re '.data[] | select(.name == "Slack Bot Token") | .id' || echo "")

existing_workflows_data=$(curl -fsSL -H "Cookie: n8n-auth=${token}" "${N8N_BASE}/rest/workflows" || echo '{"data":[]}')

for file in /mnt/workflows/*.json; do
  [ -f "$file" ] || continue

  name=$(jq -re '.name' "$file")
  existing_id=$(echo "$existing_workflows_data" | jq -r --arg name "$name" 'first(.data[] | select(.name == $name) | .id) // ""')

  if [ -n "$existing_id" ]; then
    active=$(echo "$existing_workflows_data" | jq -r --arg name "$name" 'first(.data[] | select(.name == $name) | .active) // false')
    workflow=$(jq --arg id "$existing_id" --argjson active "$active" '. + {id: $id, active: $active}' "$file")

    if [ -n "$slack_credential_id" ]; then
      workflow=$(echo "$workflow" | jq --arg id "$slack_credential_id" '
        .nodes |= map(
          if .type == "n8n-nodes-base.slack" then
            .credentials = {"slackApi": {"id": $id, "name": "Slack Bot Token"}}
          else
            .
          end
        )
      ')
    fi

    if ! echo "$workflow" | curl -fsSL -H "Cookie: n8n-auth=${token}" -H "Content-Type: application/json" -XPUT "${N8N_BASE}/rest/workflows/${existing_id}" -d @- > /dev/null; then
      echo "failed to update: $name" >&2
      continue
    fi
    echo "updated: $name"
  else
    workflow=$(jq '. + {active: false}' "$file")

    if [ -n "$slack_credential_id" ]; then
      workflow=$(echo "$workflow" | jq --arg id "$slack_credential_id" '
        .nodes |= map(
          if .type == "n8n-nodes-base.slack" then
            .credentials = {"slackApi": {"id": $id, "name": "Slack Bot Token"}}
          else
            .
          end
        )
      ')
    fi

    if ! echo "$workflow" | curl -fsSL -H "Cookie: n8n-auth=${token}" -H "Content-Type: application/json" -XPOST "${N8N_BASE}/rest/workflows" -d @- > /dev/null; then
      echo "failed to import: $name" >&2
      continue
    fi
    echo "imported: $name"
  fi
done
