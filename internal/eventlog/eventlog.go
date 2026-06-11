// Package eventlog persists session-scoped envelopes for resume and
// subscription replay. The default implementation is in-memory ring
// per session.
package eventlog

import (
	"sync"
	"time"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
)

// Entry is one persisted envelope.
type Entry struct {
	SessionID string
	EventSeq  uint64
	JobID     string
	StoredAt  time.Time
	Envelope  arcp.Envelope
}

// Log is the event-log interface.
type Log interface {
	Append(e Entry) error
	Since(sessionID string, fromSeq uint64) ([]Entry, error)
	SinceJob(jobID string, fromSeq uint64) ([]Entry, error)
	Trim(sessionID string, beforeSeq uint64) error
	// Oldest returns the smallest retained EventSeq for sessionID and
	// whether any entry is retained.
	Oldest(sessionID string) (uint64, bool)
}

// Memory is a per-session in-memory event log with a fixed retention
// window. It is not durable; for production deployments wire a
// persistent Log against your own store.
type Memory struct {
	mu     sync.Mutex
	bySess map[string][]Entry
	max    int
}

// NewMemory returns a Memory log retaining at most maxPerSession
// entries per session (the oldest are dropped past the limit).
func NewMemory(maxPerSession int) *Memory {
	if maxPerSession <= 0 {
		maxPerSession = 10_000
	}
	return &Memory{
		bySess: map[string][]Entry{},
		max:    maxPerSession,
	}
}

// Append stores e indexed by session id.
func (m *Memory) Append(e Entry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e.StoredAt.IsZero() {
		e.StoredAt = time.Now()
	}
	entries := m.bySess[e.SessionID]
	entries = append(entries, e)
	if len(entries) > m.max {
		drop := len(entries) - m.max
		entries = entries[drop:]
	}
	m.bySess[e.SessionID] = entries
	return nil
}

// Since returns entries for sessionID whose EventSeq > fromSeq.
func (m *Memory) Since(sessionID string, fromSeq uint64) ([]Entry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	src := m.bySess[sessionID]
	out := make([]Entry, 0, len(src))
	for _, e := range src {
		if e.EventSeq > fromSeq {
			out = append(out, e)
		}
	}
	return out, nil
}

// SinceJob returns entries for jobID whose EventSeq > fromSeq across
// all sessions.
func (m *Memory) SinceJob(jobID string, fromSeq uint64) ([]Entry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []Entry
	for _, entries := range m.bySess {
		for _, e := range entries {
			if e.JobID == jobID && e.EventSeq > fromSeq {
				out = append(out, e)
			}
		}
	}
	return out, nil
}

// Oldest returns the smallest retained EventSeq for sessionID. The
// second return is false when no entry is retained.
func (m *Memory) Oldest(sessionID string) (uint64, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	entries := m.bySess[sessionID]
	if len(entries) == 0 {
		return 0, false
	}
	min := entries[0].EventSeq
	for _, e := range entries[1:] {
		if e.EventSeq < min {
			min = e.EventSeq
		}
	}
	return min, true
}

// Trim drops entries for sessionID whose EventSeq <= beforeSeq.
func (m *Memory) Trim(sessionID string, beforeSeq uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	entries := m.bySess[sessionID]
	kept := entries[:0]
	for _, e := range entries {
		if e.EventSeq > beforeSeq {
			kept = append(kept, e)
		}
	}
	if len(kept) == 0 {
		// Don't leak an empty slice keyed by a fully-trimmed session.
		delete(m.bySess, sessionID)
	} else {
		m.bySess[sessionID] = kept
	}
	return nil
}
