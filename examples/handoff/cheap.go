// Cheap-tier inference. Real version: an anthropic-sdk-go call with a
// system prompt asking for a `Confidence: X.XX` line, then heuristics
// on top to derive the final score.
package main

import (
	"context"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
	"github.com/agentruntimecontrolprotocol/go-sdk/messages"
)

func attempt(context.Context, string) (string, float64) {
	panic("not implemented: attempt")
}

// Session: as-if-published high-level wrapper.
type Session struct{}

// openCheap returns the connected client + the runtime's
// session.accepted (so the caller can pin RuntimeIdentity).
func openCheap(context.Context, string) (*Session, *messages.SessionAccepted) {
	panic("not implemented: openCheap — transport, identity, auth elided")
}

func (*Session) Request(context.Context, *arcp.Envelope) (*arcp.Envelope, error) {
	panic("not implemented")
}
func (*Session) Send(context.Context, *arcp.Envelope) error { panic("not implemented") }
func (*Session) SessionID() arcp.SessionID                  { panic("not implemented") }
func (*Session) Close(context.Context) error                { return nil }
