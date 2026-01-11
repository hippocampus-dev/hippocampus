# HTTP Client Pattern

How to use http.Client in Go applications.

## Response Body Close

```go
defer func() {
    _ = response.Body.Close()
}()
```

Do NOT use `defer response.Body.Close()`.

## Status Code Check

```go
if response.StatusCode >= http.StatusBadRequest {
    defer func() {
        _ = response.Body.Close()
    }()
    body, _ := io.ReadAll(response.Body)
    return nil, xerrors.Errorf("API error: status=%d, body=%s", response.StatusCode, string(body))
}
```

For equality comparisons (`==`, `!=`), use `http.Status*` constants instead of numeric literals (`http.StatusConflict` instead of `409`). Range comparisons (`>= 500 && < 600`) may use numeric literals.

## Request Creation

| Location | Pattern |
|----------|---------|
| HTTP handler | `http.NewRequestWithContext(r.Context(), ...)` |
| Controller/function with ctx | `http.NewRequestWithContext(ctx, ...)` |
| CLI tool main | `http.NewRequest(...)` |

Do NOT use `http.Get()`, `http.Post()`, or `http.DefaultClient.Get()`.

Use `http.MethodGet`, `http.MethodPost`, etc. instead of `"GET"`, `"POST"` strings.

## Keep-Alive

Add to service main functions after `flag.Parse()`:

```go
http.DefaultTransport.(*http.Transport).MaxIdleConnsPerHost = http.DefaultTransport.(*http.Transport).MaxIdleConns
```

## Client Selection

| Condition | Use |
|-----------|-----|
| No custom TLS/Timeout/Transport | `http.DefaultClient` |
| Custom TLS, Timeout, or Transport | `&http.Client{}` |

```go
// Custom TLS
client := &http.Client{
    Transport: &http.Transport{
        TLSClientConfig: &tls.Config{RootCAs: pool},
        MaxIdleConnsPerHost: http.DefaultTransport.(*http.Transport).MaxIdleConns,
    },
}

// Custom Timeout
client := &http.Client{
    Timeout: 1 * time.Second,
    Transport: &retry.Transport{Base: http.DefaultTransport},
}
```

## API Client Structs

```go
type Client struct {
    httpClient *http.Client
}

func NewClient() *Client {
    return &Client{httpClient: http.DefaultClient}
}
```

## JSON Response Decoding

| Condition | Pattern |
|-----------|---------|
| Success response | `json.NewDecoder(response.Body).Decode(&result)` |
| Error response (need body in message) | `io.ReadAll(response.Body)` |

```go
// Success
var result Result
if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
    return nil, xerrors.Errorf("failed to decode response: %w", err)
}

// Error
if response.StatusCode >= http.StatusBadRequest {
    body, _ := io.ReadAll(response.Body)
    return nil, xerrors.Errorf("API error: status=%d, body=%s", response.StatusCode, string(body))
}
```
