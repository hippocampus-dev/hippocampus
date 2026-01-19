# Configuration Pattern

How to structure CLI configuration in Go applications.

## Environment Variable Fallback

For simple applications without complex validation needs, use the generic `envOrDefaultValue` helper for environment variable fallback on flag values:

```go
func envOrDefaultValue[T any](key string, defaultValue T) T {
    value, exists := os.LookupEnv(key)
    if !exists {
        return defaultValue
    }

    switch any(defaultValue).(type) {
    case string:
        return any(value).(T)
    case int:
        if intValue, err := strconv.Atoi(value); err == nil {
            return any(intValue).(T)
        }
    case bool:
        if boolValue, err := strconv.ParseBool(value); err == nil {
            return any(boolValue).(T)
        }
    case time.Duration:
        if durationValue, err := time.ParseDuration(value); err == nil {
            return any(durationValue).(T)
        }
    }

    return defaultValue
}
```

Usage:

```go
flag.StringVar(&address, "address", envOrDefaultValue("ADDRESS", "0.0.0.0:8080"), "HTTP server address")
flag.IntVar(&port, "port", envOrDefaultValue("PORT", 8080), "Server port")
flag.DurationVar(&timeout, "timeout", envOrDefaultValue("TIMEOUT", 10*time.Second), "Request timeout")
```

| Use Case | Pattern |
|----------|---------|
| Simple flags without validation | `envOrDefaultValue` |
| Complex validation or composition | `Args` struct + `DefaultArgs()` |

## Args Struct

Define all configuration in a struct with `validate` tags:

```go
type Args struct {
    Host        string `validate:"required,ip"`
    Port        int    `validate:"required,gt=0,lte=65535"`
    CertDir     string `validate:"required,dir"`
    MetricsAddr string `validate:"required,tcp_addr"`
    EnableHTTP2 bool
    *ParentArgs // Embed parent configuration
}
```

## DefaultArgs Function

Provide sensible defaults:

```go
func DefaultArgs() *Args {
    return &Args{
        Host:        "0.0.0.0",
        Port:        9443,
        CertDir:     "/var/k8s-webhook-server/serving-certs",
        MetricsAddr: "0.0.0.0:8080",
        EnableHTTP2: false,
        ParentArgs:  parent.DefaultArgs(),
    }
}
```

## Composition

Use struct embedding for configuration hierarchy:

```go
type WebhookArgs struct {
    *lock.Args  // Embedded lock configuration
    *http.Args  // Embedded HTTP configuration
}
```

## Validation

Validate at `Run()` entry using `go-playground/validator`:

```go
func Run(a *Args) error {
    if err := validator.New().Struct(a); err != nil {
        return xerrors.Errorf("invalid arguments: %w", err)
    }
    // ...
}
```
