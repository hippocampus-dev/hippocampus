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

| Export Style | Use |
|--------------|-----|
| `export function name()` | Named exports |
| `export default function Name()` | Default exports (React components, handlers) |
| `export const` | Non-function values (objects, constants, class instances) |
| `export const` | Builder-pattern APIs that return callable objects (e.g., `createServerFn().handler()`) |
