# Observability (§8.2, §11)

ARCP separates the wire-level concerns covered by this guide into
two pieces:

- **Trace propagation** (§11): W3C trace context rides inside
  `envelope.extensions` under the key
  `"x-vendor.opentelemetry.tracecontext"`.
- **Runtime observations** (§8.2): agents emit `metric`, `status`,
  `log`, `artifact_ref`, and `progress` events.

## Trace IDs on the wire

`Envelope.TraceID` is the canonical 32-hex W3C trace identifier on
every envelope. The Go SDK propagates it end-to-end:

```go
h, err := cli.Submit(ctx, client.SubmitRequest{
    Agent:   "echo",
    TraceID: traceID,
})
// Inside the agent:
//   jc.TraceID()           // returns the trace id from job.submit (or a freshly minted one)
//   ctx := jc.Context()    // bound to the job lifecycle, suitable for downstream calls
```

If `SubmitRequest.TraceID` is empty the runtime allocates a fresh
trace id and echoes it back on `job.accepted`.

## OpenTelemetry middleware

`middleware/otel.WrapTransport(t, opts)` returns a
`transport.Transport` that injects the active span's W3C trace
context into outbound envelopes and starts spans on inbound ones.

```go
otelT := otel.WrapTransport(t, otel.Options{
    TracerProvider: tp,
    Propagator:     propagation.TraceContext{},
    JobSpans:       true, // default when no flag is set
    ToolCallSpans:  true, // default when no flag is set
})
cli, err := client.Connect(ctx, otelT, client.Options{...})
```

`Options`:

| Field | Effect |
| --- | --- |
| `TracerProvider` | nil uses `otel.GetTracerProvider()`. |
| `Propagator` | nil uses `otel.GetTextMapPropagator()`. |
| `FrameSpans` | One short-lived span per inbound envelope, named `arcp.recv <type>`. |
| `JobSpans` | One span per inbound `job.*` envelope. |
| `ToolCallSpans` | One span per inbound `job.event` whose kind is `tool_call` or `tool_result`. |

When none of the three span flags is set, `JobSpans` and
`ToolCallSpans` default to true. `ExtensionsKey` is exported as a
constant for callers that need to read or set the carrier directly.

## Runtime observations

Agents can emit structured runtime observations as `metric`,
`status`, `log`, `artifact_ref`, and `progress` events (see
[Job events](./job-events.md) for the full table). The runtime also
emits one of these automatically: a `cost.budget.remaining` metric
fires after each `cost.*` debit when the change crosses 5% of the
initial budget.

Avoid logging credential values or other secrets. Credential IDs,
job IDs, and trace IDs are safe operational handles.
