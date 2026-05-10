package transport

import (
	"context"
	"errors"

	"github.com/fizzpop/arcp-go"
)

// Transport is the bidirectional envelope channel for one ARCP
// connection (RFC §22). Each side of a session holds a Transport.
//
// Implementations MUST honor ctx for both Send and Recv: they MUST
// return promptly when ctx.Done() is closed. Send and Recv MAY be
// called concurrently from different goroutines, but each end is
// expected to be driven by a single reader and a single writer in v0.1
// (the runtime and client both follow this discipline).
type Transport interface {
	// Send delivers env to the peer. Must serialize the envelope to
	// the wire format and apply transport-specific framing.
	Send(ctx context.Context, env arcp.Envelope) error
	// Recv blocks until the next envelope arrives, ctx is done, or the
	// transport closes. Returns io.EOF (or a wrapped equivalent) on
	// peer-initiated close.
	Recv(ctx context.Context) (arcp.Envelope, error)
	// Close terminates the transport. Subsequent Send calls return an
	// error; pending Recv calls unblock with EOF. Safe to call more
	// than once.
	Close() error
}

// ErrClosed is returned by Send when the transport has been closed.
var ErrClosed = errors.New("transport: closed")
