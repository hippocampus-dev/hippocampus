# Configuration Pattern

How to structure CLI configuration in Go applications.

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
