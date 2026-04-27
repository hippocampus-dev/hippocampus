---
paths:
  - "**/*.js"
  - "**/*.ts"
  - "**/*.tsx"
---

* Use explicit `null` checks (`=== null`) instead of truthy checks for APIs that return `null`
* Check Web IDL to determine if an API returns `null` (look for `optional` or nullable types)
* Chrome Extension content scripts: wrap in IIFE `(() => { ... })()` to avoid global scope pollution
* Use function declarations (`export function`, `export default function`) for exported functions, not arrow function expressions (`export const fn = () =>`)
* Cloudflare Worker entrypoints exporting a handler object literal append `satisfies ExportedHandler<Environment>` (e.g., `export default { email, fetch } satisfies ExportedHandler<Environment>`)
* Place a `.npmrc` with `ignore-scripts=true` at each Node.js project root; invoke build steps explicitly via `npm run build` (lifecycle scripts from dependencies are disabled for supply-chain safety)

| Export Style | Use |
|--------------|-----|
| `export function name()` | Named exports |
| `export default function Name()` | Default exports (React components, handlers) |
| `export const` | Non-function values (objects, constants, class instances) |
| `export const` | Builder-pattern APIs that return callable objects (e.g., `createServerFn().handler()`) |
| `export default { ... } satisfies ExportedHandler<Env>` | Cloudflare Worker entrypoints with multiple handlers (`email`, `fetch`, `scheduled`) |

## Reference

If writing tests:
  Read: `.claude/reference/javascript/testing.md`
