# Tracing Skip Guidelines

What arguments to skip in `#[tracing::instrument(skip(...))]`.

## What to `skip`

Always skip these argument types to protect privacy and reduce log volume:

| Category | Examples | Reason |
|----------|----------|--------|
| User input | `content`, `query`, `text` | Privacy protection |
| Request/Response | `request`, `response` | Contains headers, body data |
| Large data | `documents`, `indices`, `postings_list` | Performance, log size |
| Auth data | `token` (auth), `credentials`, `api_key` | Security |

## What NOT to Skip

Keep these for useful debugging context:

| Category | Examples | Reason |
|----------|----------|--------|
| Identifiers | `id`, `key`, `name`, `bucket`, `path` | Trace correlation |
| Control params | `limit`, `offset`, `page` | Query context |
| Enums/Booleans | `mode`, `enabled`, `strategy` | State information |
| Metadata | `generation`, `version` | Version tracking |

## Examples

```rust
// Good: Skip user content, keep identifiers
#[tracing::instrument(skip(self, content))]
async fn upload(&self, bucket: &str, key: &str, content: &[u8]) -> Result<()>

// Good: Skip query (user input)
#[tracing::instrument(skip(self, query))]
async fn search(&self, query: &Query) -> Result<Vec<Document>>

// Good: Skip large data structures
#[tracing::instrument(skip(self, documents))]
async fn index(&self, documents: Vec<Document>) -> Result<()>

// Good: Skip request object (contains headers, body)
#[tracing::instrument(skip(self, request))]
async fn handle(&self, request: Request) -> Response
```

## Pattern Summary

```
skip if: user_input || large_data || auth_sensitive || request_response
keep if: identifier || control_param || enum || boolean || metadata
```
