#!/usr/bin/env bash

set -eo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

N8N_BASE="http://n8n:5678"

if ! curl -fsSL "${N8N_BASE}/rest/settings" | jq -re '.data.isInstanceOwnerSetUp == true' > /dev/null; then
  curl -fsSL -XPOST -H "Content-Type: application/json" "${N8N_BASE}/rest/owner/setup" -d "$(jq -n --arg email "$N8N_OWNER_EMAIL" --arg firstName "$N8N_OWNER_FIRST_NAME" --arg lastName "$N8N_OWNER_LAST_NAME" --arg password "$N8N_OWNER_PASSWORD" '{
    email: $email,
    firstName: $firstName,
    lastName: $lastName,
    password: $password
  }')" > /dev/null
fi

existing_workflows=$(n8n list:workflow --json 2>/dev/null | jq -re '[.[].name]' || echo '[]')

for file in /mnt/workflows/*.json; do
  [ -f "$file" ] || continue

  name=$(jq -re '.name' "$file")

  if echo "$existing_workflows" | jq -re --arg name "$name" 'index($name) != null' > /dev/null; then
    continue
  fi

  if ! n8n import:workflow --input="$file"; then
    echo "failed to import: $name" >&2
    continue
  fi
  echo "imported: $name"
done
