package transport

import (
	"context"
	"encoding/json"
	"io"
	"sync"

	"github.com/fizzpop/arcp-go"
)

// memoryTransport is one end of an in-memory paired transport. It is
// useful for tests and for in-process runtime/client coupling. Sends
// pass through json.Marshal/Unmarshal so the in-memory transport
// exercises the wire format and surfaces serialization bugs the same
// way a real socket would.
type memoryTransport struct {
	out chan<- []byte
	in  <-chan []byte

	closed    chan struct{}
	closeOnce *sync.Once
}

// NewInMemoryPair returns two paired transports, each one end of the
// connection. Either end's Close terminates both sides; pending Recv
// calls return io.EOF. Channel capacity is bounded; senders block when
// the buffer is full.
func NewInMemoryPair() (Transport, Transport) {
	a2b := make(chan []byte, 64)
	b2a := make(chan []byte, 64)
	closed := make(chan struct{})
	var once sync.Once
	a := &memoryTransport{out: a2b, in: b2a, closed: closed, closeOnce: &once}
	b := &memoryTransport{out: b2a, in: a2b, closed: closed, closeOnce: &once}
	return a, b
}

// Send marshals env and queues it for the peer.
func (t *memoryTransport) Send(ctx context.Context, env arcp.Envelope) error {
	data, err := json.Marshal(env)
	if err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.closed:
		return ErrClosed
	case t.out <- data:
		return nil
	}
}

// Recv waits for a peer envelope, ctx cancellation, or close.
func (t *memoryTransport) Recv(ctx context.Context) (arcp.Envelope, error) {
	select {
	case <-ctx.Done():
		return arcp.Envelope{}, ctx.Err()
	case <-t.closed:
		return arcp.Envelope{}, io.EOF
	case data, ok := <-t.in:
		if !ok {
			return arcp.Envelope{}, io.EOF
		}
		var env arcp.Envelope
		if err := json.Unmarshal(data, &env); err != nil {
			return arcp.Envelope{}, err
		}
		return env, nil
	}
}

// Close terminates the paired transport. Idempotent.
func (t *memoryTransport) Close() error {
	t.closeOnce.Do(func() { close(t.closed) })
	return nil
}
