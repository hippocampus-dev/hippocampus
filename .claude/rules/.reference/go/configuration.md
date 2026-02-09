# Configuration Pattern

How to structure CLI configuration in Go applications.

## Pattern Selection

| CLI Framework | Environment Variable Pattern | Validation |
|---------------|------------------------------|------------|
| `flag` (no Cobra) | `envOrDefaultValue` at flag site | Optional |
| Cobra | `os.Getenv`/`os.LookupEnv` in `DefaultArgs()` | `validator.New().Struct(a)` at `Run()` entry |

## flag-based Applications (envOrDefaultValue)

For applications using `flag` directly without Cobra, use the `envOrDefaultValue` helper inline at flag registration:

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
    case int64:
        if intValue, err := strconv.ParseInt(value, 10, 64); err == nil {
            return any(intValue).(T)
        }
    case uint:
        if uintValue, err := strconv.ParseUint(value, 10, 0); err == nil {
            return any(uint(uintValue)).(T)
        }
    case uint64:
        if uintValue, err := strconv.ParseUint(value, 10, 64); err == nil {
            return any(uintValue).(T)
        }
    case float64:
        if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
            return any(floatValue).(T)
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

### Required Flag Validation

For required flags with no meaningful default, use `""` as default and validate after `flag.Parse()`. Include both the flag name and environment variable in the error message:

```go
flag.StringVar(&owner, "owner", envOrDefaultValue("GITHUB_OWNER", ""), "GitHub owner")
flag.Parse()

if owner == "" {
    log.Fatal("--owner or GITHUB_OWNER is required")
}
```

| Scenario | Format |
|----------|--------|
| Single required flag | `"--flag or ENV_VAR is required"` |
| Multiple alternative flags | `"either --flag-a/ENV_A, --flag-b/ENV_B, or ... are required"` |

## Cobra-based Applications (Args struct)

For Cobra applications, use `Args` struct with `DefaultArgs()`. Environment variables are resolved in `DefaultArgs()`, NOT at flag registration.

### Args Struct

Define in `pkg/{command}/args.go` with `validate` tags:

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

### DefaultArgs Function

Resolve environment variables here using `os.Getenv` (strings) or `os.LookupEnv` (non-strings needing parsing):

```go
func DefaultArgs() *Args {
    a := &Args{
        Host:        "0.0.0.0",
        Port:        9443,
        CertDir:     "/var/k8s-webhook-server/serving-certs",
        MetricsAddr: "0.0.0.0:8080",
        EnableHTTP2: false,
        ParentArgs:  parent.DefaultArgs(),
    }
    if v, ok := os.LookupEnv("PORT"); ok {
        if n, err := strconv.Atoi(v); err == nil {
            a.Port = n
        }
    }
    return a
}
```

| Type | Pattern | Example |
|------|---------|---------|
| `string` | `os.Getenv` | `SlackToken: os.Getenv("SLACK_TOKEN")` |
| Non-string | `os.LookupEnv` + parse | `if v, ok := os.LookupEnv("PORT"); ok { ... }` |

### Command Definition

Define in `cmd/{command}.go`:

```go
func serveCmd() *cobra.Command {
    serveArgs := serve.DefaultArgs()

    cmd := &cobra.Command{
        Use:          "serve",
        Short:        "Start the server",
        SilenceUsage: true,
        Args:         cobra.NoArgs,
        RunE: func(cmd *cobra.Command, args []string) error {
            if err := serve.Run(serveArgs); err != nil {
                return xerrors.Errorf("failed to run serve.Run: %w", err)
            }
            return nil
        },
    }

    cmd.Flags().StringVar(
        &serveArgs.Address,
        "address",
        serveArgs.Address,
        "Address",
    )

    return cmd
}
```

| Element | Convention |
|---------|------------|
| Args variable | `{package}Args` (e.g., `serveArgs`, `webhookArgs`) |
| Command variable | `cmd` |
| Flag registration | Multi-line (one argument per line) |
| Default value | `{package}Args.Field` (from `DefaultArgs()`) |

### Sensitive Flag Masking

Mask sensitive values (API keys, tokens) in help output by overriding `DefValue` after registration:

```go
cmd.Flags().StringVar(
    &egosearchArgs.SlackToken,
    "token",
    egosearchArgs.SlackToken,
    "Slack token",
)

if f := cmd.Flags().Lookup("token"); f != nil {
    f.DefValue = "********"
}
```

### Composition

Use struct embedding for configuration hierarchy:

```go
type WebhookArgs struct {
    *lock.Args  // Embedded lock configuration
    *http.Args  // Embedded HTTP configuration
}
```

### Validation

Validate at `Run()` entry using `go-playground/validator`:

```go
func Run(a *Args) error {
    if err := validator.New().Struct(a); err != nil {
        return xerrors.Errorf("invalid arguments: %w", err)
    }
    // ...
}
```
