package server

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
	"github.com/agentruntimecontrolprotocol/go-sdk/internal/clock"
	"github.com/agentruntimecontrolprotocol/go-sdk/internal/eventlog"
	"github.com/agentruntimecontrolprotocol/go-sdk/internal/idstore"
	"github.com/agentruntimecontrolprotocol/go-sdk/messages"
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

// TestCloseDropsResumeEntries covers #86: after Close returns, no
// resume entry can be claimed by a subsequent handshake.
func TestCloseDropsResumeEntries(t *testing.T) {
	srv := New(Options{ResumeWindow: time.Hour})
	sess := &session{srv: srv, id: "s", principal: "p", seq: srv.allocFor("s")}
	srv.stashResume(sess, "tok")
	if err := srv.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.claimResume(messages.ResumeRequest{SessionID: "s", ResumeToken: "tok"}, "p"); err == nil {
		t.Fatal("resume entry must not be claimable after Close")
	}
}

// TestCompareVersions covers #94: semver-style numeric ordering.
func TestCompareVersions(t *testing.T) {
	if compareVersions("1.10.0", "1.9.0") <= 0 {
		t.Fatal("1.10.0 must be newer than 1.9.0")
	}
	if compareVersions("2.0.0", "1.9.0") <= 0 {
		t.Fatal("2.0.0 must be newer than 1.9.0")
	}
	if compareVersions("1.0.0", "1.0.0") != 0 {
		t.Fatal("equal versions must compare 0")
	}
}

// TestInventoryBare covers #95: a bare-only agent is distinguishable.
func TestInventoryBare(t *testing.T) {
	srv := New(Options{})
	defer srv.Close()
	srv.RegisterAgent("bareonly", func(ctx context.Context, _ json.RawMessage, jc *JobContext) (any, error) { return nil, nil })
	inv := srv.inventory()
	found := false
	for _, e := range inv {
		if e.Name == "bareonly" {
			found = true
			if !e.Bare {
				t.Fatal("bare-only agent must report Bare=true in inventory")
			}
		}
	}
	if !found {
		t.Fatal("bareonly agent missing from inventory")
	}
}

// TestListJobsCursorCollision covers #85: pagination is stable when
// CreatedAt collides across jobs.
func TestListJobsCursorCollision(t *testing.T) {
	srv := New(Options{})
	defer srv.Close()
	ts := time.Now()
	for _, id := range []string{"a", "b", "c", "d"} {
		j := &Job{id: id, principal: "p", agent: "x", createdAt: ts, statusV: messages.StatusRunning,
			session: &session{srv: srv, id: "s", seq: srv.allocFor("s")}, lease: nil}
		srv.jobs[id] = j
	}
	page1, next, err := srv.listJobs("p", messages.ListJobsFilter{}, 2, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(page1) != 2 || next == "" {
		t.Fatalf("page1=%d next=%q", len(page1), next)
	}
	page2, _, err := srv.listJobs("p", messages.ListJobsFilter{}, 2, next)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, j := range append(page1, page2...) {
		if seen[j.JobID] {
			t.Fatalf("duplicate job %s across pages", j.JobID)
		}
		seen[j.JobID] = true
	}
	if len(seen) != 4 {
		t.Fatalf("expected 4 distinct jobs across pages, got %d", len(seen))
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
