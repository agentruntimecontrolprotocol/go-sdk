# tracing

Spec §11. The client wraps its transport with `middleware/otel`,
injecting a W3C trace context carrier in
`extensions["x-vendor.opentelemetry.tracecontext"]`.
