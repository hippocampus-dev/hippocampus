---
paths:
  - "**/Cargo.toml"
---

* Write every dependency as an inline table, `name = { version = "..." }` even when it carries nothing but a version (`packages/gcs/Cargo.toml`)
* Put path dependencies in a trailing block of the same dependency table, after every registry dependency and separated from them by a blank line, writing an optional one inline as `name = { path = "...", optional = true }` rather than as a `[dependencies.name]` subtable (`packages/jwt/Cargo.toml`)
