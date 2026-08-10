# resource-field-hook

<!-- TOC -->
* [resource-field-hook](#resource-field-hook)
  * [Annotation](#annotation)
  * [Expression](#expression)
  * [What Is Scaled](#what-is-scaled)
  * [Limitations](#limitations)
  * [Development](#development)
<!-- TOC -->

resource-field-hook is a webhook that scales a Downward API resource value before the container sees it.

`resourceFieldRef` hands a container the whole `limits.memory`, and neither the Downward API nor a `divisor` can express a fraction of it — `divisor` only divides, its accepted values are a fixed set of unit strings (`validContainerResourceDivisorForMemory`), and the quotient is rounded up. A `GOMEMLIMIT` sourced that way therefore equals the cgroup limit exactly, leaving Go no room for the memory its runtime does not account for.

This hook resolves the `resourceFieldRef` at admission, evaluates the expression named in the pod's annotations, and replaces `valueFrom` with the resulting byte count as a literal `value`. The two fields are mutually exclusive on an `EnvVar`, so the source cannot be kept alongside the result.

## Annotation

One annotation per environment variable of a container, read from the pod. The suffix names the container and the environment variable; the value is the expression.

```yaml
resource-field-hook.kaidotio.github.io/my-container.GOMEMLIMIT: '{{ scale .Value 0.9 }}'
resource-field-hook.kaidotio.github.io/my-sidecar.GOMEMLIMIT: '{{ sub .Value (quantity "4Mi") }}'
```

The container is named because one expression rarely suits every container in a pod: a share of the limit is right for a container large enough that a percentage leaves usable room, and wrong for the sidecar beside it that is not.
Naming it always, rather than only where the containers differ, keeps one key form to read and leaves an annotation naming no container meaning nothing rather than meaning every container.

The container name stays out of the domain because the domain states which system owns the annotation, and that is this hook whichever container the annotation is for — a domain that varied per container would stop `resource-field-hook.kaidotio.github.io` from selecting the hook's annotations as a set.
Nothing splits the suffix back apart, since both names are known when the key is built, so a container name and an environment variable name either side of a dot need no rule to tell them apart.

The two names share the annotation's 63 character suffix, which a container name of up to 63 characters can exhaust on its own. An environment variable of a container named too long to leave room for it cannot be annotated.

An environment variable no annotation names is left alone, so opting in is per container and per environment variable. See `examples/`.

## Expression

A Go `text/template` rendering to a byte count. `.Value` is what the `resourceFieldRef` resolved to, in bytes, and is the only field exposed — which resource it came from is already stated by the `resourceFieldRef` the expression is attached to.

`text/template` ships no arithmetic, so infix operators are not available: `{{ .Value }} * 0.9` renders the ` * 0.9` as literal text and fails to parse as a byte count. Use the functions instead.

| Function | Meaning |
|----------|---------|
| `scale v f` | `floor(v * f)`, for `f` above 0 and at most 1 |
| `quantity "64Mi"` | A written size as bytes |
| `sub a b` / `add a b` | Arithmetic on byte counts |
| `min a b` / `max a b` | The smaller / larger of two byte counts |

```yaml
# a share of the limit, for a container large enough that a percentage leaves usable room
'{{ scale .Value 0.9 }}'

# a fixed amount held back, for a container too small for that — 10% of 8Mi is under 1Mi,
# which the binary's own mapped pages exceed
'{{ sub .Value (quantity "4Mi") }}'

# whichever leaves more headroom, which is the share below ~640Mi and the reserve above it
'{{ min (scale .Value 0.9) (sub .Value (quantity "64Mi")) }}'
```

An expression that fails to parse, fails to evaluate, or does not render a positive whole number of bytes leaves the environment variable alone. Nothing validates it at write time, so a typo is silent apart from the log line the hook emits.

## What Is Scaled

The hook only rewrites an `EnvVar` when all of the following hold. Anything else is passed through, and the kubelet resolves the original `resourceFieldRef` as it already does.

| Condition | Reason |
|-----------|--------|
| `valueFrom.resourceFieldRef` is set | A literal value carries no source to recompute from |
| An annotation names the container holding the environment variable, and the environment variable | Opt-in |
| The resource is a memory resource | A CPU resource is a core count whose consumers round it themselves |
| The `divisor` is absent, `0`, or `1` | Reproducing the Downward API's round-up for other divisors risks disagreeing with the kubelet. `0` belongs here because the API server normalizes an omitted `divisor` to it, so a zero divisor means one rather than a division by zero |
| The referenced container states the resource | The kubelet falls back to node allocatable, which is not knowable at admission |
| The expression renders a positive byte count | A reserve larger than the limit leaves nothing to set |

`containerName` is honoured, defaulting to the container holding the environment variable. Init containers, which is how a native sidecar is declared, are covered alongside regular containers, and a container name in an annotation names either kind.

An annotation is matched against the container holding the environment variable, not against the `containerName` the `resourceFieldRef` reads from. A container that sizes itself from another's limit is still the one whose headroom the expression describes.

## Limitations

A rewritten environment variable no longer carries its `resourceFieldRef`, so a reinvocation cannot recompute it. If another mutating webhook changes `limits.memory` after this one runs, the value stays as first computed rather than following the new limit. Mutating webhooks are invoked in order of their configuration's name, so this applies to a webhook whose name sorts after `resource-field-hook` — VPA's, among others.

Do not annotate a workload whose resources a VPA rewrites at admission (`updateMode: Auto`). Leaving it unannotated keeps the plain `resourceFieldRef`, which the kubelet resolves against whatever the VPA settled on.

## Development

```sh
$ make dev
```
