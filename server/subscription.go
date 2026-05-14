package server

import (
	"context"
	"sync"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
)

type subscription struct {
	session  *session
	jobID    string
	closeOnce sync.Once
	closed    chan struct{}
}

func newSubscription(s *session, jobID string) *subscription {
	return &subscription{
		session: s,
		jobID:   jobID,
		closed:  make(chan struct{}),
	}
}

// publish forwards env to the subscriber's session, allocating a fresh
// session-scoped event_seq so the subscriber's seq space stays
// monotonic.
func (s *subscription) publish(ctx context.Context, env arcp.Envelope) {
	select {
	case <-s.closed:
		return
	default:
	}
	out := env
	out.EventSeq = s.session.nextSeq()
	s.session.send(out)
}

func (s *subscription) close() {
	s.closeOnce.Do(func() { close(s.closed) })
}
