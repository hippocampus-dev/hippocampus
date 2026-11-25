# workers

<!-- TOC -->
* [workers](#workers)
  * [Features](#features)
  * [Usage](#usage)
    * [API Routes](#api-routes)
  * [Development](#development)
  * [Deployment](#deployment)
<!-- TOC -->

workers is a code sharing service built with Next.js 16 and deployed on Cloudflare Workers.

## Features

- Syntax highlighting with Shiki
- Line numbers with click-to-copy line links
- Line highlighting via URL hash (`#L10`, `#L10-L20` format)
- Shift+click for range selection
- AI-powered code explanations (Workers AI) with caching
- Expiring pastes (1 hour, 1 day, 1 week, 1 month, or never)
- Raw text view
- Copy code and share link
- Sandboxed code execution for JavaScript and Python (Cloudflare Dynamic Workers)

## Usage

### API Routes

- `POST /api/paste` - Create a new paste
- `GET /api/paste/[id]` - Get paste metadata and content
- `GET /api/paste/[id]/raw` - Get raw paste content
- `POST /api/paste/[id]/explain` - Generate AI explanation (cached for 1 hour or until paste expires)
- `POST /api/paste/[id]/run` - Execute paste code in a sandboxed V8 isolate (JavaScript and Python)

## Development

```sh
$ make dev
```

## Deployment

Before deploying, create the required Cloudflare resources:

```sh
# Create KV namespace
wrangler kv namespace create PASTE_KV

# Create R2 bucket
wrangler r2 bucket create paste
```

Update `wrangler.jsonc` with the KV namespace ID, then deploy:

```sh
npm run deploy
```
