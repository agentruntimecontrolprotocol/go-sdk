---
title: middleware/otel
sdk: go
spec_sections: [§11]
order: 6
kind: reference
pkg_godoc: https://pkg.go.dev/github.com/agentruntimecontrolprotocol/go-sdk/middleware/otel
---

# Reference: middleware/otel

`otel.WrapTransport(t, Options)` returns a `transport.Transport` that
- injects the active span's W3C trace context into
  `envelope.extensions["x-vendor.opentelemetry.tracecontext"]` on send;
- extracts the carrier on receive and starts matching spans.

Configuration:

```go
otel.Options{
    TracerProvider: otel.GetTracerProvider(),
    Propagator:     otel.GetTextMapPropagator(),
    JobSpans:       true,
    ToolCallSpans:  true,
    FrameSpans:     false,
}
```
