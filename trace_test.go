package arcp_test

import (
	"context"
	"testing"

	"github.com/fizzpop/arcp-go"
)

func TestTraceContextPropagation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	if arcp.TraceIDFromContext(ctx) != "" {
		t.Errorf("expected empty trace id on bare context")
	}
	tid := arcp.NewTraceID()
	sid := arcp.NewSpanID()
	pid := arcp.NewSpanID()
	ctx = arcp.WithTraceID(ctx, tid)
	ctx = arcp.WithSpanID(ctx, sid)
	ctx = arcp.WithParentSpanID(ctx, pid)
	if got := arcp.TraceIDFromContext(ctx); got != tid {
		t.Errorf("trace id mismatch: %q vs %q", got, tid)
	}
	if got := arcp.SpanIDFromContext(ctx); got != sid {
		t.Errorf("span id mismatch: %q vs %q", got, sid)
	}
	if got := arcp.ParentSpanIDFromContext(ctx); got != pid {
		t.Errorf("parent span id mismatch: %q vs %q", got, pid)
	}
}

func TestTraceContextEmptyDefaults(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	if arcp.SpanIDFromContext(ctx) != "" {
		t.Errorf("expected empty span id default")
	}
	if arcp.ParentSpanIDFromContext(ctx) != "" {
		t.Errorf("expected empty parent span id default")
	}
}
