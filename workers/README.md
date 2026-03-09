# workers

<!-- TOC -->
* [workers](#workers)
  * [Features](#features)
    * [Tech Stack](#tech-stack)
    * [Architecture](#architecture)
  * [Usage](#usage)
    * [API Routes](#api-routes)
  * [Development](#development)
  * [Deployment](#deployment)
<!-- TOC -->

workers is a code sharing service built with Next.js 15 and deployed on Cloudflare Workers.

## Features

- Syntax highlighting with Shiki
- Line numbers with click-to-copy line links
- Line highlighting via URL hash (`#L10`, `#L10-L20` format)
- Shift+click for range selection
- AI-powered code explanations (Workers AI) with caching
- Expiring pastes (1 hour, 1 day, 1 week, 1 month, or never)
- Raw text view
- Copy code and share link

### Tech Stack

- **Framework**: Next.js 15 with React 19
- **UI Components**: shadcn/ui (Radix UI primitives + Tailwind CSS)
- **Styling**: Tailwind CSS v4
- **Deployment**: Cloudflare Workers via OpenNext
- **Storage**: Cloudflare R2 (content) + KV (metadata)
- **AI**: Cloudflare Workers AI (Llama 3.1)
- **Syntax Highlighting**: Shiki

### Architecture

The application uses abstraction layers for platform portability:

- **AI Provider** (`lib/ai/`): Abstracts AI chat operations for provider-agnostic code explanations
- **Storage Repository** (`lib/storage/`): Abstracts paste storage (CRUD, explanations) for different backends

Currently implements Cloudflare-specific providers. To add support for other platforms (e.g., Vercel), implement the `AiProvider` and `PasteRepository` interfaces.

## Usage

### API Routes

- `POST /api/paste` - Create a new paste
- `GET /api/paste/[id]` - Get paste metadata and content
- `GET /api/paste/[id]/raw` - Get raw paste content
- `POST /api/paste/[id]/explain` - Generate AI explanation (cached for 1 hour or until paste expires)

## Development

```sh
npm install
npm run dev
```

## Deployment

Before deploying, create the required Cloudflare resources:

```sh
# Create KV namespace
wrangler kv namespace create PASTE_KV

# Create R2 bucket
wrangler r2 bucket create paste-bucket
```

Update `wrangler.jsonc` with the KV namespace ID, then deploy:

```sh
npm run deploy
```
