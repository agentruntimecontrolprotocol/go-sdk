package server

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
	"github.com/agentruntimecontrolprotocol/go-sdk/internal/clock"
	"github.com/agentruntimecontrolprotocol/go-sdk/messages"
)

// TestDiscardStopsTimersAndCtx covers #156 and #80: a discarded job
// cancels its context and stops its expiry/max-runtime timers so no
// spurious job.error is emitted after the submission is rejected.
func TestDiscardStopsTimersAndCtx(t *testing.T) {
	mock := clock.NewMock(time.Now())
	srv := New(Options{Clock: mock})
	defer srv.Close()
	sctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sess := &session{
		srv:       srv,
		id:        "s",
		principal: "p",
		seq:       srv.allocFor("s"),
		ctx:       sctx,
		outbox:    make(chan arcp.Envelope, 16),
	}
	future := mock.Now().Add(time.Hour)
	req := messages.JobSubmit{
		Agent:            "x",
		LeaseConstraints: &messages.LeaseConstraints{ExpiresAt: &future},
		MaxRuntimeSec:    1,
	}
	fn := func(ctx context.Context, _ json.RawMessage, jc *JobContext) (any, error) { return nil, nil }
	job := newJob(sess, "x@1", req, fn, "")

	job.discard()
	if job.ctx.Err() == nil {
		t.Fatal("discard did not cancel the job context")
	}

	// Advance well past both expiry and max-runtime: the timers were
	// stopped, so nothing should be emitted for the discarded job.
	mock.Advance(2 * time.Hour)
	select {
	case env := <-sess.outbox:
		t.Fatalf("discarded job emitted %s; timers were not stopped", env.Type)
	default:
	}
}
