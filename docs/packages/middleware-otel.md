# Package `middleware/otel`

Import path: `github.com/agentruntimecontrolprotocol/go-sdk/middleware/otel`

`middleware/otel` provides OpenTelemetry helpers for ARCP runtimes and
clients. Use it to connect envelope trace IDs and W3C trace context to
your existing telemetry pipeline.

Credential values and other secrets should not be placed in spans,
logs, or metric attributes.
