// Package transport defines the Transport interface and ships three
// concrete implementations: an in-memory pair (for tests and same-
// process embedders), a WebSocket transport, and an NDJSON-over-stdio
// transport.
package transport

import (
	"context"
	"errors"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
)

// Transport is the bidirectional envelope channel for one ARCP
// connection. Each side of a session holds exactly one Transport.
//
// Implementations MUST honour ctx for both Send and Recv: both calls
// MUST return promptly when ctx.Done() is closed. Send and Recv MAY be
// called concurrently from different goroutines, but each end is
// expected to be driven by a single reader and a single writer.
type Transport interface {
	// Send delivers env to the peer, applying transport-specific
	// framing.
	Send(ctx context.Context, env arcp.Envelope) error
	// Recv blocks until the next envelope arrives, ctx is done, or the
	// transport closes.
	Recv(ctx context.Context) (arcp.Envelope, error)
	// Close terminates the transport. Subsequent Send calls return
	// ErrClosed; pending Recv calls unblock with io.EOF or an
	// equivalent wrapped error. Safe to call more than once.
	Close() error
}

// ErrClosed is returned by Send and Recv after the transport is
// closed.
var ErrClosed = errors.New("transport: closed")
