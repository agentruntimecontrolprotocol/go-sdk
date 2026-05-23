# Vendor extensions (§5, §15)

ARCP reserves the `x-vendor.*` namespace for vendor-defined fields,
event kinds, and lease capabilities (§15). Unprefixed names are
spec-reserved.

## The `Extensions` envelope field

Each envelope carries an opaque, vendor-namespaced map:

```go
type Envelope struct {
    // ... standard fields ...
    Extensions map[string]json.RawMessage `json:"extensions,omitempty"`
}
```

`Extensions` is the only top-level envelope field that round-trips
unknown data. Arbitrary unknown JSON fields *outside* the extensions
map are silently dropped by the default `encoding/json` decoder.

Read or write extensions directly:

```go
// Read on the receiver side.
if raw, ok := env.Extensions["x-vendor.acme.tenant"]; ok {
    var tenant string
    _ = json.Unmarshal(raw, &tenant)
}

// Write on the sender side.
raw, _ := json.Marshal("tenant-a")
if env.Extensions == nil {
    env.Extensions = map[string]json.RawMessage{}
}
env.Extensions["x-vendor.acme.tenant"] = raw
```

The OpenTelemetry middleware uses this mechanism to carry W3C trace
context under the key `"x-vendor.opentelemetry.tracecontext"` —
exported as `otel.ExtensionsKey`.

## Custom lease capabilities

Use `x-vendor.<name>.<capability>` for custom lease namespaces:

```go
lease := arcp.Lease{
    "x-vendor.acme.email.send": {"tenant-a/*"},
}
```

The runtime treats vendor capabilities exactly like reserved
namespaces: glob match against patterns, lease subset checks via
`arcp.IsLeaseSubset`, and `PERMISSION_DENIED` on miss.

## Custom event kinds

Custom event kinds also use the `x-vendor.<name>.<kind>` prefix.
`messages.DecodeEventBody` returns the raw `json.RawMessage` for
unknown kinds; embedders decode the body shape themselves.

Keep extension payloads JSON-compatible and document whether they are
session-scoped, job-scoped, or event-only. Do not use unprefixed
names for non-standard capabilities or event kinds.
