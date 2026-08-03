# CloudEvents Handler Pattern

How to implement CloudEvents receivers that return a reply event or forward to an HTTP sink.

## Reply datacontenttype

Set the data content type explicitly on every reply.
KafkaSource emits events with no `datacontenttype`, and the SDK registers `json.Decode` for the empty content type, so inbound `DataAs` succeeds.
But a reply returned unchanged (`return &e, cloudevents.ResultACK`) serializes as `text/plain; charset=utf-8`, which routes the next step's `DataAs` to the text decoder — it accepts only `*string` and fails.

That failure is silent: the downstream handler returns `nil` with HTTP 200, so events vanish without an error in any log.
When a Knative Sequence delivers nothing, compare the Kafka channel topic offsets of every step rather than reading logs.

| Handler | Call |
|---------|------|
| Transforms the payload | `SetData(cloudevents.ApplicationJSON, obj)` |
| Forwards the payload unchanged | `SetDataContentType(cloudevents.ApplicationJSON)` |

```go
// Transforming: re-encodes obj, returns an error
response := e.Clone()
if err := response.SetData(cloudevents.ApplicationJSON, obj); err != nil {
    return nil, cloudevents.ResultACK
}

// Forwarding: writes only the attribute, cannot fail
response := e.Clone()
response.SetDataContentType(cloudevents.ApplicationJSON)

return &response, cloudevents.ResultACK
```

Prefer `SetDataContentType` when forwarding.
`SetData` re-encodes from the decoded struct, so any field the struct does not declare is silently dropped under version skew between producer and forwarder, and its error branch discards an event that upstream state may already treat as handled.
Do not reach for `SetData(contentType, e.Data())` to preserve bytes — the `[]byte` branch sets `DataBase64`, changing the structured-mode wire representation.

Clone before mutating.
`Event.Context` is an interface holding a pointer, so a by-value handler parameter still shares it with the caller — both calls above write through to the invoker's event otherwise.

## Sink status codes

A receiver that forwards to an HTTP sink decides redelivery by its return value.
`ResultACK` consumes the event permanently, so an unlogged ACK on a rejected call is indistinguishable from success at every layer above it.

| Sink response | Return | Reason |
|---------------|--------|--------|
| Transport error, 5xx | `fmt.Errorf(...)` | Transient — redelivery can succeed |
| 4xx | Log the status and body, then `ResultACK` | Redelivery cannot fix a rejected payload, so the log is the only trace |
| 2xx, 3xx | `ResultACK` | — |

Read the sink's API version from its own documentation rather than assuming the path in an existing URL still resolves.
A removed API answers 4xx, which this table routes to the log-and-discard branch.

## Deduplication keys

A deduplicating step keys on a fixed subset of the event's fields, so a dimension added upstream splits nothing unless it reaches one of those fields.
The event still carries the new field and every step reports success, so the suppression that follows is indistinguishable from correct suppression.
Read the keying expression in the deduplicating step before adding a dimension meant to split its window.

Suppression discards the whole event, so any per-occurrence detail carried by the events that lose the key is lost with them.
Rendering such a detail in the alert body therefore attributes the whole suppressed run to whichever occurrence won the key, so word it as an example occurrence or key on the dimension it names.

## Example

Copy from: `cluster/applications/loganomaly/pkg/adapter/adapter.go` (transforming), `cluster/applications/loganomaly/pkg/deduplicator/deduplicator.go` (forwarding), `cluster/applications/cloudevents-relay/main.go` and `cluster/applications/cloudevents-alertmanager/main.go` (sink)
