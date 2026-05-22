# Observability (§11)

Trace IDs flow through envelopes and are available on both client
handles and `JobContext`.

```go
h, err := cli.Submit(ctx, client.SubmitRequest{
	Agent:   "echo",
	TraceID: traceID,
})
```

Server middleware in `middleware/otel` integrates W3C trace context with
HTTP transports. Agents can emit structured runtime observations as
`metric`, `status`, `log`, `artifact_ref`, and `progress` events.

Avoid logging credential values or other secrets. Credential IDs,
job IDs, and trace IDs are safe operational handles.
