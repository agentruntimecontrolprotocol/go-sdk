// Package clock defines the small Clock interface used by the runtime
// for testable time. Production wires real wall time; tests inject a
// Mock that they can step forward manually.
package clock

import (
	"sync"
	"time"
)

// Clock abstracts time.Now() and time.AfterFunc.
type Clock interface {
	Now() time.Time
	AfterFunc(d time.Duration, f func()) Timer
}

// Timer is the minimal subset of *time.Timer that callers need.
type Timer interface {
	Stop() bool
	Reset(d time.Duration) bool
}

// Real returns a Clock that uses wall-clock time.
func Real() Clock { return realClock{} }

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

func (realClock) AfterFunc(d time.Duration, f func()) Timer {
	return time.AfterFunc(d, f)
}

// Mock is a controllable Clock for tests.
type Mock struct {
	mu     sync.Mutex
	now    time.Time
	timers []*mockTimer
}

// NewMock returns a Mock initialised to t.
func NewMock(t time.Time) *Mock {
	return &Mock{now: t}
}

// Now returns the mocked time.
func (m *Mock) Now() time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.now
}

// Advance moves the clock forward by d, firing any expired timers.
func (m *Mock) Advance(d time.Duration) {
	m.mu.Lock()
	m.now = m.now.Add(d)
	due := []*mockTimer{}
	remaining := m.timers[:0]
	for _, t := range m.timers {
		if !t.deadline.After(m.now) && !t.stopped {
			t.stopped = true
			due = append(due, t)
		} else {
			remaining = append(remaining, t)
		}
	}
	m.timers = remaining
	m.mu.Unlock()
	for _, t := range due {
		t.f()
	}
}

// AfterFunc schedules f to fire when the mock clock reaches now+d.
func (m *Mock) AfterFunc(d time.Duration, f func()) Timer {
	m.mu.Lock()
	defer m.mu.Unlock()
	t := &mockTimer{
		mock:     m,
		deadline: m.now.Add(d),
		f:        f,
	}
	m.timers = append(m.timers, t)
	return t
}

type mockTimer struct {
	mock     *Mock
	deadline time.Time
	f        func()
	stopped  bool
}

// Stop prevents the timer from firing if it has not yet.
func (t *mockTimer) Stop() bool {
	t.mock.mu.Lock()
	defer t.mock.mu.Unlock()
	if t.stopped {
		return false
	}
	t.stopped = true
	return true
}

// Reset reschedules the timer to fire d after the current mock time.
func (t *mockTimer) Reset(d time.Duration) bool {
	t.mock.mu.Lock()
	defer t.mock.mu.Unlock()
	was := !t.stopped
	t.stopped = false
	t.deadline = t.mock.now.Add(d)
	found := false
	for _, existing := range t.mock.timers {
		if existing == t {
			found = true
			break
		}
	}
	if !found {
		t.mock.timers = append(t.mock.timers, t)
	}
	return was
}
