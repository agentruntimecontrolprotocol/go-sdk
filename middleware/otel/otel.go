// Package otel wires ARCP transports to OpenTelemetry trace context.
package otel

import (
	"context"
	"encoding/json"
	"strings"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
	"github.com/agentruntimecontrolprotocol/go-sdk/messages"
	"github.com/agentruntimecontrolprotocol/go-sdk/transport"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// ExtensionsKey is the envelope.extensions key carrying the W3C
// traceparent + tracestate carrier per spec §15.
const ExtensionsKey = "x-vendor.opentelemetry.tracecontext"

// Options configures the OTel wrapper.
type Options struct {
	TracerProvider trace.TracerProvider
	Propagator     propagation.TextMapPropagator
	FrameSpans     bool
	JobSpans       bool
	ToolCallSpans  bool
}

func (o Options) withDefaults() Options {
	if o.TracerProvider == nil {
		o.TracerProvider = otel.GetTracerProvider()
	}
	if o.Propagator == nil {
		o.Propagator = otel.GetTextMapPropagator()
	}
	if !o.JobSpans && !o.FrameSpans && !o.ToolCallSpans {
		o.JobSpans = true
		o.ToolCallSpans = true
	}
	return o
}

// WrapTransport returns a transport.Transport that propagates W3C
// trace context inside the envelope's extensions on send and starts
// matching spans on recv.
func WrapTransport(t transport.Transport, opts Options) transport.Transport {
	o := opts.withDefaults()
	return &otelTransport{inner: t, opts: o, tracer: o.TracerProvider.Tracer("arcp")}
}

type otelTransport struct {
	inner  transport.Transport
	opts   Options
	tracer trace.Tracer
}

// Send injects the active span's trace context into env.Extensions.
func (t *otelTransport) Send(ctx context.Context, env arcp.Envelope) error {
	if span := trace.SpanFromContext(ctx); span.IsRecording() {
		carrier := propagation.MapCarrier{}
		t.opts.Propagator.Inject(ctx, carrier)
		if len(carrier) > 0 {
			if env.Extensions == nil {
				env.Extensions = map[string]json.RawMessage{}
			}
			raw, err := json.Marshal(carrier)
			if err == nil {
				env.Extensions[ExtensionsKey] = raw
			}
		}
		if env.TraceID == "" {
			env.TraceID = span.SpanContext().TraceID().String()
		}
		span.SetAttributes(
			attribute.String("arcp.session_id", env.SessionID),
			attribute.String("arcp.type", env.Type),
		)
		if env.JobID != "" {
			span.SetAttributes(attribute.String("arcp.job_id", env.JobID))
		}
	}
	return t.inner.Send(ctx, env)
}

// Recv extracts trace context from env.Extensions and starts a
// matching span when configured.
//
// Spans emitted depend on Options:
//   - FrameSpans:    one short-lived span per inbound envelope
//   - JobSpans:      one span per job-typed envelope ("job.submit",
//                    "job.accepted", "job.result", "job.error",
//                    "job.cancel", "job.subscribed")
//   - ToolCallSpans: one span per inbound job.event whose Kind is
//                    "tool_call" or "tool_result"
//
// All spans are started and ended immediately so they appear as
// events on the active trace, not as long-running parents.
func (t *otelTransport) Recv(ctx context.Context) (arcp.Envelope, error) {
	env, err := t.inner.Recv(ctx)
	if err != nil {
		return env, err
	}
	if raw, ok := env.Extensions[ExtensionsKey]; ok {
		var carrier propagation.MapCarrier
		if json.Unmarshal(raw, &carrier) == nil {
			ctx = t.opts.Propagator.Extract(ctx, carrier)
		}
	}
	if t.opts.FrameSpans {
		_, span := t.tracer.Start(ctx, "arcp.recv "+env.Type)
		span.SetAttributes(
			attribute.String("arcp.session_id", env.SessionID),
			attribute.String("arcp.type", env.Type),
		)
		span.End()
	}
	if t.opts.JobSpans && strings.HasPrefix(env.Type, "job.") {
		_, span := t.tracer.Start(ctx, env.Type)
		attrs := []attribute.KeyValue{
			attribute.String("arcp.session_id", env.SessionID),
			attribute.String("arcp.type", env.Type),
		}
		if env.JobID != "" {
			attrs = append(attrs, attribute.String("arcp.job_id", env.JobID))
		}
		if env.TraceID != "" {
			attrs = append(attrs, attribute.String("arcp.trace_id", env.TraceID))
		}
		span.SetAttributes(attrs...)
		span.End()
	}
	if t.opts.ToolCallSpans && env.Type == messages.TypeJobEvent {
		var ev messages.JobEvent
		if json.Unmarshal(env.Payload, &ev) == nil {
			if ev.Kind == messages.KindToolCall || ev.Kind == messages.KindToolResult {
				_, span := t.tracer.Start(ctx, "arcp."+ev.Kind)
				span.SetAttributes(
					attribute.String("arcp.session_id", env.SessionID),
					attribute.String("arcp.job_id", env.JobID),
					attribute.String("arcp.event.kind", ev.Kind),
				)
				span.End()
			}
		}
	}
	return env, nil
}

// Close passes through.
func (t *otelTransport) Close() error { return t.inner.Close() }
