# resource-field-hook

<!-- TOC -->
* [resource-field-hook](#resource-field-hook)
  * [Usage](#usage)
    * [Functions](#functions)
  * [Development](#development)
<!-- TOC -->

resource-field-hook is a webhook that scales a Downward API resource value before the container sees it.

## Usage

Annotate a pod with one key per container and environment variable.
The value is a Go `text/template` rendering to a byte count, where `.Value` is the resolved `resourceFieldRef` in bytes.

```yaml
resource-field-hook.kaidotio.github.io/my-container.GOMEMLIMIT: '{{ scale .Value 0.9 }}'
resource-field-hook.kaidotio.github.io/my-sidecar.GOMEMLIMIT: '{{ sub .Value (quantity "4Mi") }}'
```

The hook resolves the `resourceFieldRef` at admission and replaces `valueFrom` with the result as a literal `value`.
An `EnvVar` is rewritten only when all of the following hold, and is passed through otherwise.

- `valueFrom.resourceFieldRef` is set
- An annotation names both the container holding the environment variable and the environment variable
- The resource is a memory resource
- The `divisor` is absent, `0`, or `1`
- The referenced container states the resource
- The expression renders a positive byte count

`containerName` is honoured, defaulting to the container holding the environment variable, and init containers are covered alongside regular containers.
An expression that fails to parse or evaluate leaves the environment variable alone and reports only to the log.
Do not annotate a workload whose resources a VPA rewrites at admission (`updateMode: Auto`).
See `examples/`.

### Functions

| Function | Meaning |
|----------|---------|
| `scale v f` | `floor(v * f)`, for `f` above 0 and at most 1 |
| `quantity "64Mi"` | A written size as bytes |
| `sub a b` / `add a b` | Arithmetic on byte counts |
| `min a b` / `max a b` | The smaller / larger of two byte counts |

## Development

```sh
$ make dev
```
