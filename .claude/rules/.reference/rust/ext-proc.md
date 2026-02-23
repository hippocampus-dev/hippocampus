# Envoy External Processor (ext_proc)

Patterns for developing Envoy External Processor gRPC services.

## When to Use

Use ext_proc when you need to modify headers based on body content. Proxy-wasm cannot modify headers during body phases (request or response).

| Approach | Can Modify Headers from Body |
|----------|------------------------------|
| proxy-wasm only | No |
| ext_proc | Yes |

| Use Case | Direction | Example |
|----------|-----------|---------|
| Compute hash from request body, add as request header | Request | envoy-request-hasher |
| Convert response body format, update response headers | Response | envoy-markdownify |

## Configuration

```yaml
http_filters:
  - name: envoy.filters.http.ext_proc
    typed_config:
      "@type": type.googleapis.com/envoy.extensions.filters.http.ext_proc.v3.ExternalProcessor
      grpc_service:
        envoy_grpc:
          cluster_name: ext-proc
        timeout: 5s
      processing_mode:
        request_header_mode: SEND      # Required to initiate gRPC stream
        response_header_mode: SKIP     # Skip if not processing response headers
        request_body_mode: BUFFERED    # Required to receive body
        response_body_mode: NONE
        request_trailer_mode: SKIP
        response_trailer_mode: SKIP
```

### Stream Initiation

`request_header_mode: SEND` is required even if you only process the body. When set to `SKIP`, Envoy does not open the gRPC stream at all.

| Mode | Stream Opened | Body Processing |
|------|---------------|-----------------|
| `request_header_mode: SEND` | Yes | Works |
| `request_header_mode: SKIP` | No | Does not work |

### Mode Selection

Prefer `SKIP` for phases the ext-proc server does not need to process. This reduces unnecessary gRPC round-trips.

| Phase Needed | Mode | Result |
|--------------|------|--------|
| Yes | `SEND` | Processed by ext-proc |
| No | `SKIP` | Envoy skips ext-proc for this phase (recommended) |
| No | `SEND` | Works but wasteful (ext-proc returns empty response) |

## Implementation Pattern

ext_proc receives body chunks via `ProcessingRequest::RequestBody` or `ProcessingRequest::ResponseBody`. When `end_of_stream` is true, compute the value and return `BodyResponse` with `header_mutation` (and optionally `body_mutation` for response body replacement).

### Defensive Response Pattern

Always respond to all request types, even with empty responses. Using `_ => {}` causes 504 timeouts when EnvoyFilter is misconfigured:

| Pattern | EnvoyFilter Mismatch | Result |
|---------|---------------------|--------|
| `_ => {}` | Envoy sends unexpected phase | 504 timeout |
| Explicit default responses | Envoy sends unexpected phase | Request continues |

```rust
match request.request {
    Some(processing_request::Request::RequestHeaders(_)) => {
        yield ProcessingResponse {
            response: Some(processing_response::Response::RequestHeaders(
                HeadersResponse::default(),
            )),
            ..Default::default()
        };
    }
    Some(processing_request::Request::RequestBody(body)) => {
        hasher.update(&body.body);

        if body.end_of_stream {
            let hash = hex::encode(hasher.finalize_reset());

            yield ProcessingResponse {
                response: Some(processing_response::Response::RequestBody(
                    BodyResponse {
                        response: Some(CommonResponse {
                            header_mutation: Some(HeaderMutation {
                                set_headers: vec![HeaderValueOption {
                                    header: Some(HeaderValue {
                                        key: "x-body-hash".to_string(),
                                        raw_value: hash.into_bytes(),
                                        ..Default::default()
                                    }),
                                    ..Default::default()
                                }],
                                ..Default::default()
                            }),
                            ..Default::default()
                        }),
                    },
                )),
                ..Default::default()
            };
        }
    }
    // Defensive: respond to all phases to prevent 504 on EnvoyFilter misconfiguration
    Some(processing_request::Request::ResponseHeaders(_)) => {
        yield ProcessingResponse {
            response: Some(processing_response::Response::ResponseHeaders(
                HeadersResponse::default(),
            )),
            ..Default::default()
        };
    }
    Some(processing_request::Request::ResponseBody(_)) => {
        yield ProcessingResponse {
            response: Some(processing_response::Response::ResponseBody(
                BodyResponse::default(),
            )),
            ..Default::default()
        };
    }
    Some(processing_request::Request::RequestTrailers(_)) => {
        yield ProcessingResponse {
            response: Some(processing_response::Response::RequestTrailers(
                TrailersResponse::default(),
            )),
            ..Default::default()
        };
    }
    Some(processing_request::Request::ResponseTrailers(_)) => {
        yield ProcessingResponse {
            response: Some(processing_response::Response::ResponseTrailers(
                TrailersResponse::default(),
            )),
            ..Default::default()
        };
    }
    None => {}
}
```

## HeaderValue Field Selection

Envoy 1.25+ uses `raw_value` (bytes) instead of `value` (string). Setting both fields simultaneously causes internal server errors on Envoy 1.31+.

| Field | Envoy 1.24 | Envoy 1.25+ | Notes |
|-------|------------|-------------|-------|
| `value` | Works | Silently ignored (ISE if both set on 1.31+) | String field (deprecated) |
| `raw_value` | Silently ignored | Works | Bytes field |

### Writing Headers (Setting)

Use only `raw_value`. Do NOT set both `value` and `raw_value`:

```rust
envoy_grpc_ext_proc::envoy::config::core::v3::HeaderValue {
    key: "x-header".to_string(),
    raw_value: b"header-value".to_vec(),
    ..Default::default()
}
```

### Reading Headers (Incoming)

When reading incoming headers from Envoy, prefer `raw_value` when non-empty, falling back to `value`:

```rust
fn header_value(header: &envoy_grpc_ext_proc::envoy::config::core::v3::HeaderValue) -> &str {
    if header.raw_value.is_empty() {
        &header.value
    } else {
        std::str::from_utf8(&header.raw_value).unwrap_or("")
    }
}
```

Usage:

```rust
if let Some(accept) = headers.headers.as_ref().and_then(|h| {
    h.headers.iter().find(|header| header.key == "accept")
}) {
    if accepts_markdown(header_value(accept)) {
        should_convert = true;
    }
}
```

Example: `cluster/applications/envoy-request-hasher/`, `cluster/applications/envoy-markdownify/`
