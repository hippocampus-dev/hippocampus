# Envoy External Processor (ext_proc)

Patterns for developing Envoy External Processor gRPC services.

## When to Use

Use ext_proc when you need to modify request headers based on request body content. Proxy-wasm cannot modify headers during body phase.

| Approach | Can Modify Headers from Body |
|----------|------------------------------|
| proxy-wasm only | No |
| ext_proc | Yes |

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

ext_proc receives body chunks via `ProcessingRequest::RequestBody`. When `end_of_stream` is true, compute the value and return `BodyResponse` with `header_mutation`.

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

| Field | Works | Notes |
|-------|-------|-------|
| `value` | No | Silently ignored by Envoy ext_proc |
| `raw_value` | Yes | Use this field for setting header values |

Envoy's `HeaderValue` protobuf has both `value` (deprecated string) and `raw_value` (bytes). The `value` field is silently ignored in ext_proc responses - headers are not set. Always use the `raw_value` field with `.into_bytes()` for string values.

Example: `cluster/applications/proxy-wasm/ext-proc/`
