---
paths:
  - "workers/**"
---

* Next.js on Cloudflare Workers via opennextjs-cloudflare
* Local development (`npm run dev`) supports all Cloudflare bindings (KV, R2, AI) via remote bindings
* Do NOT suggest `npm run preview` for binding access - `npm run dev` works
* Do NOT use `getCloudflareContext` directly in route handlers or Server Components
* Place reusable components in `components/` (not co-located with route segments in `app/`)
* Uses shadcn/ui components in `components/ui/`

## Abstraction Layer

Use Repository pattern for storage and Provider pattern for AI.

| Context | Pattern |
|---------|---------|
| Route handlers / Server Components | `await get*()` factory functions from `lib/` |
| Custom workers (direct env access) | Import `Cloudflare*` classes from `lib/*/cloudflare` |

Factory functions are async and use `getCloudflareContext({ async: true })` internally for SSR/SSG compatibility.

## Storage

| Storage | When to Use |
|---------|-------------|
| KV | Small data (<25MB), fast reads, TTL support |
| R2 | Large content, binary data |

## Syntax Highlighting

Use Shiki with `createJavaScriptRegexEngine()` for Edge compatibility (no Oniguruma WASM).
