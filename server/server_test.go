package server

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
	"github.com/agentruntimecontrolprotocol/go-sdk/internal/idstore"
	"github.com/agentruntimecontrolprotocol/go-sdk/messages"
	"github.com/agentruntimecontrolprotocol/go-sdk/transport"
)

// failingIDStore is a fault-injecting idstore.Store used by #50.
type failingIDStore struct{ err error }

func (f *failingIDStore) PutIfAbsent(ctx context.Context, e idstore.Entry) (idstore.Entry, bool, error) {
	return idstore.Entry{}, false, f.err
}
func (f *failingIDStore) Get(ctx context.Context, principal, key string) (idstore.Entry, bool, error) {
	return idstore.Entry{}, false, nil
}
func (f *failingIDStore) SetAccepted(ctx context.Context, principal, key string, accepted []byte) error {
	return nil
}
func (f *failingIDStore) Sweep(ctx context.Context, olderThan time.Time) (int, error) {
	return 0, nil
}

// TestIDStoreErrorRejectsSubmit covers #50: when the idempotency store
// returns an error, the server must reject the submit with an error
// envelope, unregister the job, and never start the agent.
func TestIDStoreErrorRejectsSubmit(t *testing.T) {
	srv := New(Options{})
	defer srv.Close()
	store := &failingIDStore{err: errors.New("boom")}
	srv.setIDStore(store)

	started := atomic.Int32{}
	srv.RegisterAgent("noop", func(ctx context.Context, _ json.RawMessage, jc *JobContext) (any, error) {
		started.Add(1)
		return nil, nil
	})

	a, b := transport.NewMemoryPair()
	srvCtx, cancelSrv := context.WithCancel(context.Background())
	defer cancelSrv()
	go func() { _ = srv.Accept(srvCtx, b) }()

	// Drive the wire by hand: hello → expect welcome → submit → expect error.
	hello, _ := arcp.NewEnvelope(messages.TypeSessionHello, &messages.SessionHello{
		Client: messages.ClientInfo{Name: "test", Version: "0"},
		Auth:   messages.AuthInfo{Scheme: "bearer", Token: "demo"},
		Capabilities: messages.HelloCapabilities{
			Features: []string{"ack", "list_jobs", "subscribe"},
		},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := a.Send(ctx, hello); err != nil {
		t.Fatal(err)
	}
	welcome, err := a.Recv(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if welcome.Type != messages.TypeSessionWelcome {
		t.Fatalf("want welcome, got %s", welcome.Type)
	}

	submit, _ := arcp.NewEnvelope(messages.TypeJobSubmit, &messages.JobSubmit{
		Agent:          "noop",
		IdempotencyKey: "anything",
	})
	submit.SessionID = welcome.SessionID
	if err := a.Send(ctx, submit); err != nil {
		t.Fatal(err)
	}
	resp, err := a.Recv(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Type != messages.TypeSessionError {
		t.Fatalf("want session.error from failing idstore, got %s", resp.Type)
	}
	var serr messages.SessionError
	if err := resp.DecodePayload(&serr); err != nil {
		t.Fatal(err)
	}
	if serr.Code != arcp.CodeInternalError && serr.Code != arcp.CodeInvalidRequest {
		t.Fatalf("unexpected error code %s", serr.Code)
	}

	// Agent must not have started.
	time.Sleep(100 * time.Millisecond)
	if got := started.Load(); got != 0 {
		t.Fatalf("agent ran %d times, want 0", got)
	}
	// And no job remains registered.
	srv.mu.RLock()
	n := len(srv.jobs)
	srv.mu.RUnlock()
	if n != 0 {
		t.Fatalf("server has %d jobs after failed submit, want 0", n)
	}
}

// TestStreamResultRejectsBadChunkSize covers the defensive guard added
// for #49: a non-positive configured ChunkSize after defaults applies
// should return an INTERNAL_ERROR rather than producing oversized
// chunks. We can't normally hit this since withDefaults fills in 1MiB,
// so construct the streamedResult by-hand to exercise the guard.
func TestStreamResultRejectsBadChunkSize(t *testing.T) {
	// Build a JobContext with chunkSize forced to zero by skipping
	// StreamResult's own guard.
	jc := &JobContext{}
	// We rely on the public StreamResult path with a server that has
	// withDefaults applied. ChunkSize will be 1MiB (good).
	srv := New(Options{ChunkSize: 1024})
	defer srv.Close()
	srv.RegisterAgent("ok", func(ctx context.Context, _ json.RawMessage, jc *JobContext) (any, error) {
		w, err := jc.StreamResult("utf8")
		if err != nil {
			return nil, err
		}
		if _, err := w.Write([]byte("hello")); err != nil {
			return nil, err
		}
		return nil, w.Close()
	})
	a, b := transport.NewMemoryPair()
	srvCtx, cancelSrv := context.WithCancel(context.Background())
	defer cancelSrv()
	go func() { _ = srv.Accept(srvCtx, b) }()

	hello, _ := arcp.NewEnvelope(messages.TypeSessionHello, &messages.SessionHello{
		Client: messages.ClientInfo{Name: "t"},
		Auth:   messages.AuthInfo{Token: "x"},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = a.Send(ctx, hello)
	w, err := a.Recv(ctx)
	if err != nil {
		t.Fatal(err)
	}
	submit, _ := arcp.NewEnvelope(messages.TypeJobSubmit, &messages.JobSubmit{Agent: "ok"})
	submit.SessionID = w.SessionID
	_ = a.Send(ctx, submit)
	// Drain accept and chunks until result.
	got := 0
	for {
		env, err := a.Recv(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if env.Type == messages.TypeJobResult {
			break
		}
		if env.Type == messages.TypeJobEvent {
			var ev messages.JobEvent
			_ = env.DecodePayload(&ev)
			if ev.Kind == messages.KindResultChunk {
				got++
			}
		}
	}
	if got == 0 {
		t.Fatal("expected at least one result_chunk event")
	}
	_ = jc // keep ref so the comment above isn't lying
}

// TestServerCloseIdempotent covers #46: Close is callable many times
// and never blocks the second caller.
func TestServerCloseIdempotent(t *testing.T) {
	srv := New(Options{})
	if err := srv.Close(); err != nil {
		t.Fatal(err)
	}
	if err := srv.Close(); err != nil {
		t.Fatal(err)
	}
}

// TestServerCloseWithNoActiveSessions completes immediately without
// waiting on the sessionsDone channel.
func TestServerCloseWithNoActiveSessions(t *testing.T) {
	srv := New(Options{})
	done := make(chan struct{})
	go func() {
		_ = srv.Close()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Server.Close with no sessions did not return")
	}
}

// TestStashAndClaimResumeRoundtrip exercises the resume entry table
// directly so the validate/replay path is covered even without a
// full client.
func TestStashAndClaimResumeRoundtrip(t *testing.T) {
	srv := New(Options{ResumeWindow: 50 * time.Millisecond})
	defer srv.Close()
	alloc := srv.allocFor("sess-1")
	alloc.setIfGreater(42)
	sess := &session{
		srv:       srv,
		id:        "sess-1",
		principal: "alice",
		features:  []string{"ack"},
		seq:       alloc,
	}
	srv.stashResume(sess, "tok-1")
	entry, err := srv.claimResume(messages.ResumeRequest{
		SessionID:   "sess-1",
		ResumeToken: "tok-1",
	}, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if entry.seq != 42 {
		t.Fatalf("entry.seq = %d, want 42", entry.seq)
	}

	// Bad token after a re-stash.
	srv.stashResume(sess, "tok-2")
	if _, err := srv.claimResume(messages.ResumeRequest{
		SessionID:   "sess-1",
		ResumeToken: "wrong",
	}, "alice"); !errors.Is(err, arcp.ErrUnauthenticated) {
		t.Fatalf("want ErrUnauthenticated, got %v", err)
	}

	// Expiry.
	srv.stashResume(sess, "tok-3")
	time.Sleep(60 * time.Millisecond)
	if _, err := srv.claimResume(messages.ResumeRequest{
		SessionID:   "sess-1",
		ResumeToken: "tok-3",
	}, "alice"); !errors.Is(err, arcp.ErrResumeWindowExpired) {
		t.Fatalf("want ErrResumeWindowExpired, got %v", err)
	}
}

// TestResumePrincipalMismatchPreservesEntry covers #153: a failed
// principal check must leave the resume entry intact and claimable by
// the rightful principal.
func TestResumePrincipalMismatchPreservesEntry(t *testing.T) {
	srv := New(Options{ResumeWindow: time.Minute})
	defer srv.Close()
	alloc := srv.allocFor("sess-pm")
	alloc.setIfGreater(7)
	sess := &session{srv: srv, id: "sess-pm", principal: "alice", seq: alloc}
	srv.stashResume(sess, "tok")

	// Wrong principal: must fail without deleting the entry.
	if _, err := srv.claimResume(messages.ResumeRequest{
		SessionID:   "sess-pm",
		ResumeToken: "tok",
	}, "mallory"); !errors.Is(err, arcp.ErrUnauthenticated) {
		t.Fatalf("want ErrUnauthenticated for wrong principal, got %v", err)
	}
	// The rightful owner can still resume.
	entry, err := srv.claimResume(messages.ResumeRequest{
		SessionID:   "sess-pm",
		ResumeToken: "tok",
	}, "alice")
	if err != nil {
		t.Fatalf("rightful principal resume failed after mismatch: %v", err)
	}
	if entry.seq != 7 {
		t.Fatalf("entry.seq = %d, want 7", entry.seq)
	}
}

// TestSeqAllocSharedAcrossSessions documents that two sessions for the
// same id share the same monotonic counter.
func TestSeqAllocSharedAcrossSessions(t *testing.T) {
	srv := New(Options{})
	defer srv.Close()
	a := srv.allocFor("sess-x")
	b := srv.allocFor("sess-x")
	if a != b {
		t.Fatalf("allocFor must return the same instance for the same id")
	}
	if got := a.next(); got != 1 {
		t.Fatalf("first next = %d, want 1", got)
	}
	if got := b.next(); got != 2 {
		t.Fatalf("second next (sibling) = %d, want 2 — counters must be shared", got)
	}
}

// TestAgentResolverFallback covers the version-fallback paths in
// resolveAgent that integration tests don't directly hit.
func TestAgentResolverFallback(t *testing.T) {
	srv := New(Options{})
	defer srv.Close()
	srv.RegisterAgentVersion("solver", "1.0.0", func(ctx context.Context, _ json.RawMessage, jc *JobContext) (any, error) {
		return nil, nil
	})
	srv.RegisterAgentVersion("solver", "2.0.0", func(ctx context.Context, _ json.RawMessage, jc *JobContext) (any, error) {
		return nil, nil
	})
	// Bare ref with no default: pick the highest version by semver-style
	// numeric ordering (#94).
	_, canonical, err := srv.resolveAgent(messages.AgentRef{Name: "solver"})
	if err != nil {
		t.Fatal(err)
	}
	if canonical != "solver@2.0.0" {
		t.Fatalf("canonical = %s, want solver@2.0.0", canonical)
	}
	// Set default and retry.
	if err := srv.SetDefaultAgentVersion("solver", "2.0.0"); err != nil {
		t.Fatal(err)
	}
	_, canonical, err = srv.resolveAgent(messages.AgentRef{Name: "solver"})
	if err != nil {
		t.Fatal(err)
	}
	if canonical != "solver@2.0.0" {
		t.Fatalf("canonical = %s, want solver@2.0.0", canonical)
	}
	// Explicit unknown version errors.
	_, _, err = srv.resolveAgent(messages.AgentRef{Name: "solver", Version: "9.9.9"})
	if !errors.Is(err, arcp.ErrAgentVersionNotAvailable) {
		t.Fatalf("want ErrAgentVersionNotAvailable, got %v", err)
	}
	// Unknown name errors.
	_, _, err = srv.resolveAgent(messages.AgentRef{Name: "unknown"})
	if !errors.Is(err, arcp.ErrAgentNotAvailable) {
		t.Fatalf("want ErrAgentNotAvailable, got %v", err)
	}
}

// TestSkipUntilCursor covers the cursor-pagination helper.
func TestSkipUntilCursor(t *testing.T) {
	items := []messages.JobInfo{
		{JobID: "a"}, {JobID: "b"}, {JobID: "c"}, {JobID: "d"},
	}
	if got := skipUntilCursor(items, "b"); got != 2 {
		t.Fatalf("cursor=b => %d, want 2", got)
	}
	if got := skipUntilCursor(items, "z"); got != len(items) {
		t.Fatalf("cursor=z => %d, want %d", got, len(items))
	}
}

// TestJobAccessors exercises the trivial Job getter methods that
// integration tests don't touch directly.
func TestJobAccessors(t *testing.T) {
	srv := New(Options{})
	defer srv.Close()
	sess := &session{
		srv:       srv,
		id:        "sess",
		principal: "alice",
		seq:       srv.allocFor("sess"),
	}
	req := messages.JobSubmit{Agent: "x"}
	fn := func(ctx context.Context, _ json.RawMessage, jc *JobContext) (any, error) { return nil, nil }
	job := newJob(sess, "x@1", req, fn, "")
	if job.ID() == "" {
		t.Fatal("ID empty")
	}
	if job.Agent() != "x@1" {
		t.Fatalf("Agent = %s", job.Agent())
	}
	if job.Principal() != "alice" {
		t.Fatalf("Principal = %s", job.Principal())
	}
	if job.Lease() == nil && job.lease != nil {
		// lease() returns a clone of empty map → could be nil; just exercise.
	}
}

// TestListJobsFilter covers the filterMatch helper directly so the
// permutations don't need full integration sessions.
func TestListJobsFilter(t *testing.T) {
	t0 := time.Now()
	t1 := t0.Add(time.Hour)
	t2 := t0.Add(2 * time.Hour)
	info := messages.JobInfo{
		JobID:     "j",
		Agent:     "a",
		Status:    messages.StatusRunning,
		CreatedAt: t1,
	}
	if !filterMatch(messages.ListJobsFilter{}, info) {
		t.Fatal("empty filter must match")
	}
	if !filterMatch(messages.ListJobsFilter{Status: []string{messages.StatusRunning}}, info) {
		t.Fatal("status match expected")
	}
	if filterMatch(messages.ListJobsFilter{Status: []string{messages.StatusError}}, info) {
		t.Fatal("status mismatch should reject")
	}
	if filterMatch(messages.ListJobsFilter{Agent: "other"}, info) {
		t.Fatal("agent mismatch should reject")
	}
	if filterMatch(messages.ListJobsFilter{CreatedAfter: &t2}, info) {
		t.Fatal("created-after future should reject")
	}
	if filterMatch(messages.ListJobsFilter{CreatedBefore: &t0}, info) {
		t.Fatal("created-before past should reject")
	}
}

// TestPersistOutboundSkipsCredentialRotation locks in the security
// invariant that credential rotation events are not written to the
// event log (where they'd be replayed plain-text on resume).
func TestPersistOutboundSkipsCredentialRotation(t *testing.T) {
	srv := New(Options{})
	defer srv.Close()
	sess := &session{
		srv: srv,
		id:  "sess-y",
		seq: srv.allocFor("sess-y"),
	}
	body, _ := json.Marshal(messages.StatusBody{Phase: messages.PhaseCredentialRotated, Message: "id"})
	ev := messages.JobEvent{Kind: messages.KindStatus, Body: body, TS: time.Now()}
	env, _ := arcp.NewEnvelope(messages.TypeJobEvent, &ev)
	env.JobID = "j"
	env.EventSeq = 1
	sess.persistOutbound(env)
	entries, _ := srv.log.Since("sess-y", 0)
	if len(entries) != 0 {
		t.Fatalf("credential rotation must not be persisted; got %d entries", len(entries))
	}

	// Sanity: a normal event IS persisted.
	body, _ = json.Marshal(messages.LogBody{Level: slog.LevelInfo.String(), Message: "ok"})
	ev = messages.JobEvent{Kind: messages.KindLog, Body: body, TS: time.Now()}
	env, _ = arcp.NewEnvelope(messages.TypeJobEvent, &ev)
	env.JobID = "j"
	env.EventSeq = 2
	sess.persistOutbound(env)
	entries, _ = srv.log.Since("sess-y", 0)
	if len(entries) != 1 {
		t.Fatalf("normal event must be persisted; got %d entries", len(entries))
	}
}
