package arcp

import "context"

type contextKey int

const (
	traceIDKey contextKey = iota
	spanIDKey
	parentSpanIDKey
)

// WithTraceID returns a child context that carries the given trace id
// (RFC §17.1). Pass through this helper rather than putting the id on
// arbitrary keys so cross-package middleware can lift it.
func WithTraceID(ctx context.Context, id TraceID) context.Context {
	return context.WithValue(ctx, traceIDKey, id)
}

// TraceIDFromContext returns the trace id stored on ctx, or the empty
// id if none is present.
func TraceIDFromContext(ctx context.Context) TraceID {
	if v, ok := ctx.Value(traceIDKey).(TraceID); ok {
		return v
	}
	return ""
}

// WithSpanID returns a child context that carries the given span id
// (RFC §17.1).
func WithSpanID(ctx context.Context, id SpanID) context.Context {
	return context.WithValue(ctx, spanIDKey, id)
}

// SpanIDFromContext returns the span id stored on ctx, or the empty
// id if none is present.
func SpanIDFromContext(ctx context.Context) SpanID {
	if v, ok := ctx.Value(spanIDKey).(SpanID); ok {
		return v
	}
	return ""
}

// WithParentSpanID returns a child context that carries the given
// parent span id (RFC §17.1).
func WithParentSpanID(ctx context.Context, id SpanID) context.Context {
	return context.WithValue(ctx, parentSpanIDKey, id)
}

// ParentSpanIDFromContext returns the parent span id stored on ctx, or
// the empty id if none is present.
func ParentSpanIDFromContext(ctx context.Context) SpanID {
	if v, ok := ctx.Value(parentSpanIDKey).(SpanID); ok {
		return v
	}
	return ""
}
