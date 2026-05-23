# Package `middleware/otel`

Import path: `github.com/agentruntimecontrolprotocol/go-sdk/middleware/otel`

`middleware/otel` wraps a `transport.Transport` so it propagates W3C
trace context inside the envelope's `extensions` field and emits
matching spans.

```go
otelT := otel.WrapTransport(t, otel.Options{
    TracerProvider: tp,
    Propagator:     propagation.TraceContext{},
    JobSpans:       true,
    ToolCallSpans:  true,
})
```

## API

| Symbol | Purpose |
| --- | --- |
| `WrapTransport(t, opts) transport.Transport` | Sole entry point; returns a transport you can hand to `client.Connect` or `server.Accept`. |
| `ExtensionsKey` | Constant string `"x-vendor.opentelemetry.tracecontext"` — the envelope extensions key carrying the propagator carrier. |
| `Options.TracerProvider` | `trace.TracerProvider`; nil uses `otel.GetTracerProvider()`. |
| `Options.Propagator` | `propagation.TextMapPropagator`; nil uses `otel.GetTextMapPropagator()`. |
| `Options.FrameSpans` | One short-lived span per inbound envelope, named `arcp.recv <type>`. |
| `Options.JobSpans` | One span per inbound `job.*` envelope. |
| `Options.ToolCallSpans` | One span per inbound `job.event` whose `Kind` is `tool_call` or `tool_result`. |

When none of the three span flags is set, `JobSpans` and
`ToolCallSpans` default to true. The wrapper is symmetric: wrap the
transport on either side of the connection (or both) and trace
context flows in both directions.

Credential values and other secrets should not be placed in spans,
logs, or metric attributes.

See [guides/observability.md](../guides/observability.md) for
end-to-end usage.
