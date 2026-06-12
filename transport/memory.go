package transport

import (
	"context"
	"sync"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
)

// NewMemoryPair returns two transports connected via in-memory
// channels. Send on one returns Recv on the other. Useful for
// same-process tests and embedders.
func NewMemoryPair() (Transport, Transport) {
	a := make(chan arcp.Envelope, 64)
	b := make(chan arcp.Envelope, 64)
	closed := make(chan struct{})
	var once sync.Once
	closer := func() error {
		once.Do(func() { close(closed) })
		return nil
	}
	left := &memoryTransport{out: a, in: b, closed: closed, close: closer}
	right := &memoryTransport{out: b, in: a, closed: closed, close: closer}
	return left, right
}

type memoryTransport struct {
	out    chan<- arcp.Envelope
	in     <-chan arcp.Envelope
	closed chan struct{}
	close  func() error
}

// Send delivers env to the peer.
func (t *memoryTransport) Send(ctx context.Context, env arcp.Envelope) error {
	select {
	case <-t.closed:
		return ErrClosed
	default:
	}
	select {
	case t.out <- env:
		return nil
	case <-t.closed:
		return ErrClosed
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Recv blocks for the next envelope.
func (t *memoryTransport) Recv(ctx context.Context) (arcp.Envelope, error) {
	// Close is signalled solely by the shared closed channel; the in
	// channel is never closed, so there is no separate !ok branch.
	select {
	case env := <-t.in:
		return env, nil
	case <-t.closed:
		return arcp.Envelope{}, ErrClosed
	case <-ctx.Done():
		return arcp.Envelope{}, ctx.Err()
	}
}

// Close shuts down both ends.
func (t *memoryTransport) Close() error {
	return t.close()
}
