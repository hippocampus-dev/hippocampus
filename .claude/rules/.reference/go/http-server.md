# HTTP Server Pattern

How to implement HTTP servers in Go applications.

## Error Responses

Use `http.StatusText()` for error response messages, not `err.Error()`:

```go
// Good: Use http.StatusText for error responses
http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)

// Bad: Do not expose internal error details
http.Error(w, err.Error(), http.StatusInternalServerError)
```

| Response Type | Pattern |
|---------------|---------|
| Error response | `http.Error(w, http.StatusText(status), status)` |
| Success response | `w.Write([]byte(http.StatusText(http.StatusOK)))` |

This prevents leaking internal error details to clients while providing consistent, human-readable messages.

## Health Check Endpoint

```go
mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/plain; charset=utf-8")
    w.WriteHeader(http.StatusOK)
    _, _ = w.Write([]byte(http.StatusText(http.StatusOK)))
})
```

## Graceful Shutdown

```go
listener, err := net.Listen("tcp", address)
if err != nil {
    log.Fatalf("failed to listen: %+v", err)
}

server := &http.Server{Handler: mux}
server.SetKeepAlivesEnabled(keepAlive)

go func() {
    defer func() {
        if err := recover(); err != nil {
            log.Printf("panic: %+v\n%s", err, debug.Stack())
        }
    }()
    if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
        log.Fatalf("failed to listen: %+v", err)
    }
}()

quit := make(chan os.Signal, 1)
signal.Notify(quit, syscall.SIGTERM)
<-quit
time.Sleep(lameduck)

ctx, cancel := context.WithTimeout(context.Background(), terminationGracePeriod)
defer cancel()

if err := server.Shutdown(ctx); err != nil {
    log.Fatalf("failed to shutdown: %+v", err)
}
```

| Component | Purpose |
|-----------|---------|
| `net.Listen` + `server.Serve` | Separate listener for controlled shutdown |
| Panic recovery in goroutine | Prevent silent crashes |
| `syscall.SIGTERM` | Kubernetes termination signal |
| Lameduck period | Drain in-flight requests |
| `server.Shutdown` | Graceful connection close |

## Route Registration

Use method-prefixed patterns (Go 1.22+):

```go
mux.HandleFunc("GET /metrics", metricsHandler)
mux.HandleFunc("GET /healthz", healthzHandler)
mux.HandleFunc("POST /api/v1/resource", createHandler)
```

## Example

Copy from: `cluster/applications/exporter-merger/main.go`
