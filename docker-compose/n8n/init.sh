#!/bin/bash

set -e

if curl -fsSL http://n8n:5678/rest/owner | grep -q "email"; then
  exit 0
fi

curl -fsSL -XPOST -H "Content-Type: application/json" http://n8n:5678/rest/owner/setup -d "$(jq -n --arg email $N8N_OWNER_EMAIL --arg firstName $N8N_OWNER_FIRST_NAME --arg lastName $N8N_OWNER_LAST_NAME --arg password $N8N_OWNER_PASSWORD '{
  email: $email,
  firstName: $firstName,
  lastName: $lastName,
  password: $password
}')" > /dev/null
