# workers

<!-- TOC -->
* [workers](#workers)
  * [Features](#features)
    * [Tech Stack](#tech-stack)
    * [Architecture](#architecture)
    * [Security](#security)
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

### Tech Stack

- **Framework**: Next.js 16 with React 19
- **UI Components**: shadcn/ui (Radix UI primitives + Tailwind CSS)
- **Styling**: Tailwind CSS v4
- **Deployment**: Cloudflare Workers via OpenNext
- **Storage**: Cloudflare R2 (content) + KV (metadata)
- **AI**: Cloudflare Workers AI (Llama 3.1)
- **Code Execution**: Cloudflare Dynamic Workers (V8 isolates) + Durable Objects (async buffer)
- **Syntax Highlighting**: Shiki

### Architecture

The application uses abstraction layers for platform portability:

- **AI Provider** (`lib/ai/`): Abstracts AI chat operations for provider-agnostic code explanations
- **Storage Repository** (`lib/storage/`): Abstracts paste storage (CRUD, explanations) for different backends
- **Code Runner** (`lib/runner/`): Abstracts sandboxed code execution; dynamic worker isolates deliver tail events to a `DynamicWorkerTail` WorkerEntrypoint (re-exported from `custom-worker.ts`) via the `tails` binding, which forwards logs and exceptions to the Buffer Durable Object keyed by session ID

Currently implements Cloudflare-specific providers.
To add support for other platforms (e.g., Vercel), implement the `AiProvider` and `PasteRepository` interfaces.

### Security

`middleware.ts` issues a Content-Security-Policy on every request that reaches the Worker and clears its matcher (`/api/*`, `/_next/static/*`, `/_next/image` and `/favicon.ico` are skipped) whose `script-src` accepts nothing but that request's nonce and `'strict-dynamic'`, which makes every route server-rendered on demand because Next.js can stamp that nonce only while it renders a request.
Anything under `public/` is served straight from Cloudflare's asset store while `wrangler.jsonc` leaves `run_worker_first` unset, so no policy reaches it.
`experimental.sri` adds `integrity` hashes to the framework and polyfill scripts, while the per-route client chunks stay uncovered because Next.js omits `integrity` on the script tags it builds from the client reference manifest.
`style-src` keeps `'unsafe-inline'` since sonner appends a `<style>` element at runtime and Shiki emits `style` attributes.

## Usage

### API Routes

- `POST /api/paste` - Create a new paste
- `GET /api/paste/[id]` - Get paste metadata and content
- `GET /api/paste/[id]/raw` - Get raw paste content
- `POST /api/paste/[id]/explain` - Generate AI explanation (cached for 1 hour or until paste expires)
- `POST /api/paste/[id]/run` - Execute paste code in a sandboxed V8 isolate (JavaScript and Python)

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
