# Go Testing Patterns

How to write consistent tests in Go applications.

## Table-Driven Test Structure

Use nested `in` struct for complex inputs:

```go
func TestDispatcher_Handle(t *testing.T) {
    type in struct {
        kubernetes   kubernetes.Interface
        gitHubClient *github.Client
        request      *AlertManagerRequest
    }

    tests := []struct {
        name            string
        in              in
        wantErrorString string
    }{
        {
            "success case",
            in{fakeClient, nil, &AlertManagerRequest{}},
            "",
        },
    }
    for _, tt := range tests {
        name := tt.name
        in := tt.in
        wantErrorString := tt.wantErrorString
        t.Run(name, func(t *testing.T) {
            t.Parallel()
            // test logic
        })
    }
}
```

## Error Assertion Patterns

| Condition | Pattern |
|-----------|---------|
| Static error messages | `go-cmp` exact match |
| Dynamic error messages (URLs, timestamps) | `strings.Contains` |

### go-cmp Pattern

```go
if err == nil {
    if diff := cmp.Diff(wantErrorString, ""); diff != "" {
        t.Errorf("(-want +got):\n%s", diff)
    }
} else {
    if diff := cmp.Diff(wantErrorString, err.Error()); diff != "" {
        t.Errorf("(-want +got):\n%s", diff)
    }
}
```

### strings.Contains Pattern

Use only when errors contain dynamic parts:

```go
if err == nil {
    if wantErrorString != "" {
        t.Errorf("expected error containing %q, got nil", wantErrorString)
    }
} else {
    if wantErrorString == "" {
        t.Errorf("unexpected error: %v", err)
    } else if !strings.Contains(err.Error(), wantErrorString) {
        t.Errorf("error %q does not contain %q", err.Error(), wantErrorString)
    }
}
```

## Type Checking

Use `fmt.Sprintf("%T", v)` instead of custom type helpers:

```go
if diff := cmp.Diff(wantType, fmt.Sprintf("%T", got)); diff != "" {
    t.Errorf("type mismatch (-want +got):\n%s", diff)
}
```

Do NOT use custom type helper functions.

## Key Practices

| Practice | Reason |
|----------|--------|
| Bind loop variables before `t.Run()` | Avoid closure issues |
| Use `t.Parallel()` | Faster test execution |
| Check both nil and non-nil error cases | Prevent false positives |
| Use `go-cmp` as default | Readable diffs |
