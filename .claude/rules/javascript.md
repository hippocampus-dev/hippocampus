---
paths:
  - "**/*.js"
  - "**/*.ts"
  - "**/*.tsx"
  - "**/package.json"
---

* Use explicit `null` checks (`=== null`) instead of truthy checks for APIs that return `null`
* Check Web IDL to determine if an API returns `null` (look for `optional` or nullable types)
* Chrome Extension content scripts: wrap in IIFE `(() => { ... })()` to avoid global scope pollution
* Use function declarations (`export function`, `export default function`) for exported functions, not arrow function expressions (`export const fn = () =>`) - default-exported Preact components and hooks keep `const Name = () => {}` with a trailing `export default Name` instead
* Leave a checked-out submodule (`.gitmodules`) or anything `biome.jsonc`'s `files.includes` subtracts (`node_modules/`, `dist/`, `*.d.ts`, `*.gen.*`) to the conventions its upstream chose - the `paths` above glob them in, so a sweep or an exclusion counted against them reports a different denominator than one that subtracts them first
* Cloudflare Worker entrypoints exporting a handler object literal append `satisfies ExportedHandler<Environment>` (e.g., `export default { email, fetch } satisfies ExportedHandler<Environment>`)
* Place a `.npmrc` with `ignore-scripts=true` at each Node.js project root; invoke build steps explicitly via `npm run build` (lifecycle scripts from dependencies are disabled for supply-chain safety)
* Call `npm --no-ignore-scripts run <name>` wherever a `pre<name>`/`post<name>` hook has to fire - `ignore-scripts=true` suppresses the package's own lifecycle hooks as well, so a plain `npm run <name>` skips them without any diagnostic, and the flag reopens nothing beyond that invocation because npm exports it to the script as an empty `npm_config_ignore_scripts`, which a nested npm reads as absent and falls back to the `.npmrc` value

| Export Style | Use |
|--------------|-----|
| `export function name()` | Named exports |
| `export default function Name()` | Default-exported functions (React components) |
| `const Name = () => {}` + `export default Name` | Default-exported Preact components and hooks |
| `export default <class, value, or framework factory call>` | Not governed here - the framework or the value's own shape fixes the form (e.g. `defineConfig({...})`, `SlackFunction(...)`, `defineEventHandler(...)`) |
| `export const` | Non-function values (objects, constants, class instances) |
| `export const` | Builder-pattern APIs that return callable objects (e.g., `createServerFn().handler()`) |
| `export default { ... } satisfies ExportedHandler<Env>` | Cloudflare Worker entrypoints with multiple handlers (`email`, `fetch`, `scheduled`) |

## Reference

If writing tests:
  Read: `.claude/reference/javascript/testing.md`
