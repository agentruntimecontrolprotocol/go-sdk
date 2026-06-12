// Package idstore implements the (principal, idempotency_key) → job_id
// dedupe map. Implementations may persist; the default is in-memory
// with a TTL sweep.
package idstore

import (
	"context"
	"encoding/json"
	"sync"
	"time"
)

// Entry is one stored mapping. ParamsHash is a canonical hash of the
// original submit parameters (used to distinguish identical retries from
// conflicting key reuse, §7.2) and Accepted caches the original
// job.accepted payload so identical retries can be replayed.
type Entry struct {
	Principal  string
	Key        string
	JobID      string
	CreatedAt  time.Time
	ParamsHash string
	Accepted   json.RawMessage
}

// Store is the dedupe interface. PutIfAbsent inserts entry and returns
// the stored entry plus true if the row is new; if a row already
// exists, it returns the existing entry and false. SetAccepted records
// the original job.accepted payload for an existing entry so identical
// retries can replay it.
type Store interface {
	PutIfAbsent(ctx context.Context, e Entry) (Entry, bool, error)
	Get(ctx context.Context, principal, key string) (Entry, bool, error)
	SetAccepted(ctx context.Context, principal, key string, accepted []byte) error
	Sweep(ctx context.Context, olderThan time.Time) (int, error)
}

// Memory is an in-memory Store with a 24h default TTL.
type Memory struct {
	mu  sync.Mutex
	m   map[string]Entry
	ttl time.Duration
	now func() time.Time
}

// NewMemory returns a Memory store with the given TTL. Zero ttl uses
// 24 hours. The default-timestamp clock is wall time; inject a
// deterministic source with SetClock.
func NewMemory(ttl time.Duration) *Memory {
	if ttl == 0 {
		ttl = 24 * time.Hour
	}
	return &Memory{
		m:   map[string]Entry{},
		ttl: ttl,
		now: time.Now,
	}
}

// SetClock overrides the source used to fill a zero Entry.CreatedAt so
// library callers can avoid ambient time.Now and make dedupe/sweep tests
// deterministic (#62). A nil argument is ignored.
func (s *Memory) SetClock(now func() time.Time) {
	if now == nil {
		return
	}
	s.mu.Lock()
	s.now = now
	s.mu.Unlock()
}

// PutIfAbsent inserts e if no entry exists for (principal, key).
func (s *Memory) PutIfAbsent(ctx context.Context, e Entry) (Entry, bool, error) {
	if err := ctx.Err(); err != nil {
		return Entry{}, false, err
	}
	k := s.key(e.Principal, e.Key)
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.m[k]; ok {
		return existing, false, nil
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = s.now()
	}
	s.m[k] = e
	return e, true, nil
}

// SetAccepted records the original job.accepted payload for an existing
// (principal, key) entry. It is a no-op if the entry is absent.
func (s *Memory) SetAccepted(ctx context.Context, principal, key string, accepted []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	k := s.key(principal, key)
	s.mu.Lock()
	defer s.mu.Unlock()
	if e, ok := s.m[k]; ok {
		e.Accepted = append(json.RawMessage(nil), accepted...)
		s.m[k] = e
	}
	return nil
}

// Get returns the entry for (principal, key) if present.
func (s *Memory) Get(ctx context.Context, principal, key string) (Entry, bool, error) {
	if err := ctx.Err(); err != nil {
		return Entry{}, false, err
	}
	k := s.key(principal, key)
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.m[k]
	return e, ok, nil
}

// Sweep removes entries older than the cutoff.
func (s *Memory) Sweep(ctx context.Context, olderThan time.Time) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for k, e := range s.m {
		if e.CreatedAt.Before(olderThan) {
			delete(s.m, k)
			n++
		}
	}
	return n, nil
}

func (s *Memory) key(principal, k string) string {
	return principal + "\x00" + k
}
