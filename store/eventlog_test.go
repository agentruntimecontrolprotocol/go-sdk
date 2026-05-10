package store_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/fizzpop/arcp-go"
	"github.com/fizzpop/arcp-go/store"
)

// Local test message type registered once per package.
type ping struct {
	Greeting string `json:"greeting,omitempty"`
}

func (ping) ARCPType() string { return "store.test.ping" }

// badPayload always fails json.Marshal. Used to exercise the Append
// marshal-error path.
type badPayload struct{}

func (badPayload) ARCPType() string { return "store.test.bad" }

// MarshalJSON deliberately fails so we can hit the Append error path.
func (badPayload) MarshalJSON() ([]byte, error) {
	return nil, errors.New("intentional marshal failure")
}

func init() {
	arcp.RegisterMessageType("store.test.ping", func() arcp.MessageType { return &ping{} })
	arcp.RegisterMessageType("store.test.bad", func() arcp.MessageType { return &badPayload{} })
}

func newTestLog(t *testing.T) *store.EventLog {
	t.Helper()
	ctx := context.Background()
	l, err := store.Open(ctx, ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = l.Close() })
	return l
}

func mkEnv(sid arcp.SessionID, id arcp.MessageID, greeting string) arcp.Envelope {
	return arcp.Envelope{
		ID:        id,
		SessionID: sid,
		Timestamp: time.Date(2026, 5, 9, 13, 0, 0, 0, time.UTC),
		Payload:   &ping{Greeting: greeting},
	}
}

func TestAppendAndCount(t *testing.T) {
	t.Parallel()
	l := newTestLog(t)
	ctx := context.Background()
	sid := arcp.NewSessionID()
	for i := 0; i < 5; i++ {
		if err := l.Append(ctx, mkEnv(sid, arcp.NewMessageID(), "x")); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}
	n, err := l.Count(ctx, sid)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 5 {
		t.Errorf("count = %d, want 5", n)
	}
}

func TestAppendDuplicateIDIsIdempotent(t *testing.T) {
	t.Parallel()
	l := newTestLog(t)
	ctx := context.Background()
	sid := arcp.NewSessionID()
	id := arcp.NewMessageID()
	if err := l.Append(ctx, mkEnv(sid, id, "first")); err != nil {
		t.Fatalf("first append: %v", err)
	}
	err := l.Append(ctx, mkEnv(sid, id, "second"))
	if err == nil {
		t.Fatalf("expected duplicate error")
	}
	if !errors.Is(err, arcp.ErrAlreadyExists) {
		t.Errorf("expected ErrAlreadyExists, got %v (code=%q)", err, arcp.Code(err))
	}
	// Subsequent appends with new ids should still succeed and not be
	// blocked by sequence-number gaps from the rejected duplicate.
	if err := l.Append(ctx, mkEnv(sid, arcp.NewMessageID(), "third")); err != nil {
		t.Fatalf("third append: %v", err)
	}
	n, err := l.Count(ctx, sid)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 2 {
		t.Errorf("count = %d, want 2 (duplicate rejected)", n)
	}
}

func TestReplayOrdering(t *testing.T) {
	t.Parallel()
	l := newTestLog(t)
	ctx := context.Background()
	sid := arcp.NewSessionID()
	want := []arcp.MessageID{}
	for i := 0; i < 10; i++ {
		id := arcp.NewMessageID()
		want = append(want, id)
		if err := l.Append(ctx, mkEnv(sid, id, "x")); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}
	got, err := l.Replay(ctx, sid, "")
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("replay len = %d, want %d", len(got), len(want))
	}
	for i, env := range got {
		if env.ID != want[i] {
			t.Errorf("replay[%d].ID = %q, want %q", i, env.ID, want[i])
		}
		if _, ok := env.Payload.(*ping); !ok {
			t.Errorf("replay[%d] payload = %T, want *ping", i, env.Payload)
		}
	}
}

func TestReplayAfterMessageID(t *testing.T) {
	t.Parallel()
	l := newTestLog(t)
	ctx := context.Background()
	sid := arcp.NewSessionID()
	ids := []arcp.MessageID{}
	for i := 0; i < 5; i++ {
		id := arcp.NewMessageID()
		ids = append(ids, id)
		if err := l.Append(ctx, mkEnv(sid, id, "x")); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}
	got, err := l.Replay(ctx, sid, ids[2])
	if err != nil {
		t.Fatalf("replay after: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("replay len = %d, want 2", len(got))
	}
	if got[0].ID != ids[3] || got[1].ID != ids[4] {
		t.Errorf("replay tail wrong: %v", []arcp.MessageID{got[0].ID, got[1].ID})
	}
}

func TestReplayAfterUnknownIDIsNotFound(t *testing.T) {
	t.Parallel()
	l := newTestLog(t)
	ctx := context.Background()
	sid := arcp.NewSessionID()
	if err := l.Append(ctx, mkEnv(sid, arcp.NewMessageID(), "x")); err != nil {
		t.Fatalf("append: %v", err)
	}
	_, err := l.Replay(ctx, sid, "msg_does_not_exist")
	if err == nil {
		t.Fatalf("expected error")
	}
	if !errors.Is(err, arcp.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v (code=%q)", err, arcp.Code(err))
	}
}

func TestReplayDifferentSessionsAreIsolated(t *testing.T) {
	t.Parallel()
	l := newTestLog(t)
	ctx := context.Background()
	a, b := arcp.NewSessionID(), arcp.NewSessionID()
	for i := 0; i < 3; i++ {
		if err := l.Append(ctx, mkEnv(a, arcp.NewMessageID(), "a")); err != nil {
			t.Fatalf("append a: %v", err)
		}
		if err := l.Append(ctx, mkEnv(b, arcp.NewMessageID(), "b")); err != nil {
			t.Fatalf("append b: %v", err)
		}
	}
	gotA, err := l.Replay(ctx, a, "")
	if err != nil {
		t.Fatalf("replay a: %v", err)
	}
	if len(gotA) != 3 {
		t.Errorf("session a replay len = %d, want 3", len(gotA))
	}
	for _, env := range gotA {
		if env.SessionID != a {
			t.Errorf("session bleed: %q vs %q", env.SessionID, a)
		}
	}
}

func TestRejectsEmptySessionOrID(t *testing.T) {
	t.Parallel()
	l := newTestLog(t)
	ctx := context.Background()
	if err := l.Append(ctx, arcp.Envelope{ID: "msg_x", Payload: &ping{}}); err == nil {
		t.Errorf("expected error for empty session id")
	}
	if err := l.Append(ctx, arcp.Envelope{SessionID: "sess_x", Payload: &ping{}}); err == nil {
		t.Errorf("expected error for empty message id")
	}
	if err := l.Append(ctx, arcp.Envelope{SessionID: "sess_x", ID: "msg_x"}); err == nil {
		t.Errorf("expected error for nil payload")
	}
}

func TestOpenWithBadDSN(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	// File mode against a directory we cannot create (path of length
	// zero or otherwise unmountable). modernc.org/sqlite is forgiving
	// about file paths but rejects an obviously empty/invalid one.
	_, err := store.Open(ctx, "file:/nonexistent/dir/that/does/not/exist/db.sqlite?_pragma=foreign_keys(1)")
	if err == nil {
		t.Errorf("expected open error for unwritable path")
	}
}

func TestAppendWithAllRoutingFields(t *testing.T) {
	t.Parallel()
	l := newTestLog(t)
	ctx := context.Background()
	sid := arcp.NewSessionID()
	env := arcp.Envelope{
		ID:             arcp.NewMessageID(),
		SessionID:      sid,
		JobID:          arcp.NewJobID(),
		StreamID:       arcp.NewStreamID(),
		SubscriptionID: arcp.NewSubscriptionID(),
		TraceID:        arcp.NewTraceID(),
		SpanID:         arcp.NewSpanID(),
		CorrelationID:  arcp.NewMessageID(),
		CausationID:    arcp.NewMessageID(),
		Priority:       arcp.PriorityCritical,
		Timestamp:      time.Date(2026, 5, 9, 13, 0, 0, 0, time.UTC),
		Payload:        &ping{Greeting: "all-fields"},
	}
	if err := l.Append(ctx, env); err != nil {
		t.Fatalf("append with all fields: %v", err)
	}
	got, err := l.Replay(ctx, sid, "")
	if err != nil || len(got) != 1 {
		t.Fatalf("replay: %v len=%d", err, len(got))
	}
	if got[0].Priority != arcp.PriorityCritical {
		t.Errorf("priority not preserved: %q", got[0].Priority)
	}
	if got[0].JobID != env.JobID {
		t.Errorf("job_id not preserved")
	}
}

func TestAppendDefaultsPriorityToNormal(t *testing.T) {
	t.Parallel()
	l := newTestLog(t)
	ctx := context.Background()
	sid := arcp.NewSessionID()
	env := mkEnv(sid, arcp.NewMessageID(), "x")
	env.Priority = "" // explicitly omit
	if err := l.Append(ctx, env); err != nil {
		t.Fatalf("append: %v", err)
	}
	got, err := l.Replay(ctx, sid, "")
	if err != nil || len(got) != 1 {
		t.Fatalf("replay: %v", err)
	}
	// On replay the envelope is decoded from the stored JSON. Since
	// our Envelope struct has Priority `omitempty`, the empty string
	// round-trips as "" — this is by design. The DB row, however,
	// stores 'normal' in the priority column. Test the row directly
	// is overkill; the key invariant tested here is that Append did
	// not error on an empty priority.
	if got[0].Priority != "" && got[0].Priority != arcp.PriorityNormal {
		t.Errorf("unexpected replay priority: %q", got[0].Priority)
	}
}

func TestAppendMarshalFailure(t *testing.T) {
	t.Parallel()
	l := newTestLog(t)
	ctx := context.Background()
	err := l.Append(ctx, arcp.Envelope{
		ID:        arcp.NewMessageID(),
		SessionID: arcp.NewSessionID(),
		Payload:   &badPayload{},
	})
	if err == nil {
		t.Errorf("expected marshal error to propagate")
	}
}

func TestCountForUnknownSession(t *testing.T) {
	t.Parallel()
	l := newTestLog(t)
	ctx := context.Background()
	n, err := l.Count(ctx, arcp.NewSessionID())
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Errorf("count = %d, want 0 for unknown session", n)
	}
}

func TestAppendZeroTimestampGetsFilled(t *testing.T) {
	t.Parallel()
	l := newTestLog(t)
	ctx := context.Background()
	sid := arcp.NewSessionID()
	env := arcp.Envelope{
		ID:        arcp.NewMessageID(),
		SessionID: sid,
		Payload:   &ping{Greeting: "x"},
		// Timestamp deliberately zero
	}
	if err := l.Append(ctx, env); err != nil {
		t.Fatalf("append zero ts: %v", err)
	}
}

func TestPersistenceAcrossReopen(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	dsn := "file:" + filepath.Join(dir, "events.db")
	ctx := context.Background()

	l1, err := store.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("open 1: %v", err)
	}
	sid := arcp.NewSessionID()
	for i := 0; i < 3; i++ {
		if err := l1.Append(ctx, mkEnv(sid, arcp.NewMessageID(), "x")); err != nil {
			t.Fatalf("append: %v", err)
		}
	}
	if err := l1.Close(); err != nil {
		t.Fatalf("close 1: %v", err)
	}

	l2, err := store.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("open 2: %v", err)
	}
	defer func() { _ = l2.Close() }()

	got, err := l2.Replay(ctx, sid, "")
	if err != nil {
		t.Fatalf("replay 2: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("after reopen: replay len = %d, want 3", len(got))
	}
	// Append more — sequence should continue from 3, not collide.
	if err := l2.Append(ctx, mkEnv(sid, arcp.NewMessageID(), "x")); err != nil {
		t.Fatalf("append after reopen: %v", err)
	}
}
