package server

import (
	"context"
	"testing"
	"time"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
	"github.com/agentruntimecontrolprotocol/go-sdk/internal/clock"
	"github.com/agentruntimecontrolprotocol/go-sdk/internal/eventlog"
	"github.com/agentruntimecontrolprotocol/go-sdk/internal/idstore"
)

// TestIdempotencySweeperReclaimsExpiredKeys covers #154: the janitor
// removes idempotency entries older than the TTL with no submit/resume
// traffic, so resubmission with a long-idle key is no longer rejected.
func TestIdempotencySweeperReclaimsExpiredKeys(t *testing.T) {
	mock := clock.NewMock(time.Now())
	srv := New(Options{Clock: mock})
	defer srv.Close()

	_, fresh, err := srv.idStore.PutIfAbsent(context.Background(), idstore.Entry{
		Principal: "alice", Key: "k", JobID: "j", CreatedAt: mock.Now(),
	})
	if err != nil || !fresh {
		t.Fatalf("PutIfAbsent fresh=%v err=%v", fresh, err)
	}
	// Advance past the 24h TTL; the janitor timer fires during Advance.
	mock.Advance(25 * time.Hour)
	if _, ok, _ := srv.idStore.Get(context.Background(), "alice", "k"); ok {
		t.Fatal("expired idempotency key was not swept")
	}
}

// TestJanitorPurgesExpiredResumes covers #155: expired resume entries
// and their event logs are reclaimed by the janitor without any resume
// attempt.
func TestJanitorPurgesExpiredResumes(t *testing.T) {
	mock := clock.NewMock(time.Now())
	srv := New(Options{Clock: mock, ResumeWindow: time.Hour})
	defer srv.Close()

	alloc := srv.allocFor("sess-j")
	alloc.setIfGreater(3)
	_ = srv.log.Append(eventlog.Entry{SessionID: "sess-j", EventSeq: 1, Envelope: arcp.Envelope{}})
	sess := &session{srv: srv, id: "sess-j", principal: "alice", seq: alloc}
	srv.stashResume(sess, "tok")

	mock.Advance(2 * time.Hour)

	srv.resumeMu.Lock()
	_, ok := srv.resumes["sess-j"]
	srv.resumeMu.Unlock()
	if ok {
		t.Fatal("expired resume entry was not purged by janitor")
	}
	entries, _ := srv.log.Since("sess-j", 0)
	if len(entries) != 0 {
		t.Fatalf("expired session log not trimmed; %d entries remain", len(entries))
	}
}

// TestDropResumeTrimsLog covers #155(1): a graceful close trims the
// session's buffered event log.
func TestDropResumeTrimsLog(t *testing.T) {
	srv := New(Options{})
	defer srv.Close()
	_ = srv.log.Append(eventlog.Entry{SessionID: "s", EventSeq: 1, Envelope: arcp.Envelope{}})
	srv.dropResume("s")
	entries, _ := srv.log.Since("s", 0)
	if len(entries) != 0 {
		t.Fatalf("dropResume did not trim the log; %d entries remain", len(entries))
	}
}
