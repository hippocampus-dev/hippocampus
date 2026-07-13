# agent

<!-- TOC -->

- [agent](#agent)
  - [Features](#features)
  - [Requirements](#requirements)
  - [Usage](#usage)
  - [Development](#development)
  - [Deployment](#deployment)

<!-- TOC -->

agent is a human-in-the-loop email reply worker built on Cloudflare Agents SDK.

## Features

- Receives email via Cloudflare Email Routing at `agent@kaidotio.dev`
- Generates a reply draft with Workers AI (`@cf/meta/llama-3.2-3b-instruct`)
- Posts the draft to Slack `#email` with Approve / Edit / Deny buttons
- Edit opens a Slack modal pre-filled with the draft; saving rotates the nonce and refreshes the original message via `chat.update`
- Sends the reply to arbitrary recipients via the `send_email` Workers binding
- Persists drafts in a Durable Object SQLite table with 24h auto-expiry
- Verifies Slack request signatures (HMAC-SHA256) and one-time action nonces

## Requirements

- Sender domain `kaidotio.dev` registered in Cloudflare Email Service (SPF/DKIM/DMARC auto-configured)
- Email Routing rule for `agent@kaidotio.dev` → `email-agent` (more specific than existing `*@kaidotio.dev` rule)
- Custom domain route `agent.kaidotio.dev` pointing to the worker
- Slack App with:
  - Bot token scope `chat:write`
  - Interactivity enabled, Request URL `https://agent.kaidotio.dev/slack/interactive`
  - Signing secret
  - Channel `#email` with the bot invited
  - Editing uses `views.open` / `chat.update` — no extra scopes beyond `chat:write`
- Wrangler secrets (`wrangler secret put` is blocked by PreToolUse hook; set via Cloudflare Dashboard):
  - `SLACK_BOT_TOKEN` — Slack bot token
  - `SLACK_SIGNING_SECRET` — Slack signing secret
  - `SLACK_CHANNEL_ID` — channel ID for `#email`
  - `SENDER_ALLOWLIST` — comma-separated lowercased email addresses permitted to trigger drafts

## Usage

Send an email from an allowlisted sender to `agent@kaidotio.dev`. A draft appears in Slack `#email` within seconds. Click **Approve** to send the reply to the sender, **Edit** to revise the draft in a modal, or **Deny** to dismiss. Unresolved drafts auto-expire after 24 hours.

## Development

```sh
$ export CLOUDFLARE_API_TOKEN=<token>
$ make dev
```

Local `wrangler dev` uses Miniflare simulators for Durable Objects. The `AI` binding calls production Workers AI (no local simulator available).

## Deployment

Deployment is automated via `.github/workflows/20_agent.yaml` on pushes to `main` that touch `agent/**`. Do NOT run `wrangler deploy` manually — the PreToolUse hook blocks it.
