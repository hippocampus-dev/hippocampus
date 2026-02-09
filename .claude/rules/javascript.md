---
paths:
  - "**/*.js"
---

* Use explicit `null` checks (`=== null`) instead of truthy checks for APIs that return `null`
* Check Web IDL to determine if an API returns `null` (look for `optional` or nullable types)
* Chrome Extension content scripts: wrap in IIFE `(() => { ... })()` to avoid global scope pollution
