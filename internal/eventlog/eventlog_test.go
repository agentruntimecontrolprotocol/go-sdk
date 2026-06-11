package eventlog

import (
	"testing"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
)

func entry(seq uint64, sessID, jobID string) Entry {
	return Entry{
		SessionID: sessID,
		EventSeq:  seq,
		JobID:     jobID,
		Envelope:  arcp.Envelope{Type: "job.event", EventSeq: seq, JobID: jobID, SessionID: sessID},
	}
}

func TestAppendSinceAndTrim(t *testing.T) {
	m := NewMemory(0)
	for i := uint64(1); i <= 5; i++ {
		if err := m.Append(entry(i, "s", "j")); err != nil {
			t.Fatal(err)
		}
	}
	got, err := m.Since("s", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("Since(2) = %d entries, want 3", len(got))
	}
	if err := m.Trim("s", 3); err != nil {
		t.Fatal(err)
	}
	got, _ = m.Since("s", 0)
	if len(got) != 2 {
		t.Fatalf("after Trim(3): %d entries, want 2", len(got))
	}
}

// TestTrimAllRemovesMapKey covers #155(3): trimming every entry must
// not leave an empty slice keyed by the session id.
func TestTrimAllRemovesMapKey(t *testing.T) {
	m := NewMemory(0)
	for i := uint64(1); i <= 3; i++ {
		_ = m.Append(entry(i, "gone", "j"))
	}
	if err := m.Trim("gone", ^uint64(0)); err != nil {
		t.Fatal(err)
	}
	m.mu.Lock()
	_, ok := m.bySess["gone"]
	m.mu.Unlock()
	if ok {
		t.Fatal("fully-trimmed session still leaks a map key")
	}
}

func TestSinceJob(t *testing.T) {
	m := NewMemory(0)
	_ = m.Append(entry(1, "s1", "ja"))
	_ = m.Append(entry(2, "s2", "ja"))
	_ = m.Append(entry(3, "s1", "jb"))
	got, _ := m.SinceJob("ja", 0)
	if len(got) != 2 {
		t.Fatalf("SinceJob(ja) = %d, want 2", len(got))
	}
}

func TestRingDropsOldEntries(t *testing.T) {
	m := NewMemory(3)
	for i := uint64(1); i <= 5; i++ {
		_ = m.Append(entry(i, "s", "j"))
	}
	got, _ := m.Since("s", 0)
	if len(got) != 3 {
		t.Fatalf("Memory(3) keeps %d entries, want 3", len(got))
	}
	if got[0].EventSeq != 3 {
		t.Fatalf("oldest kept seq = %d, want 3", got[0].EventSeq)
	}
}
