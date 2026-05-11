// Per-peer client construction. Real version: a marketplace catalog
// keyed on peer name → transport URL + auth scheme + identity.
package main

import (
	"context"
	"encoding/json"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
	"github.com/agentruntimecontrolprotocol/go-sdk/messages"
)

// Session: as-if-published high-level wrapper.
type Session struct{}

// openPeer returns the client + the negotiated session.accepted +
// the extension capability blob (so the caller can build a Profile).
func openPeer(context.Context, string) (
	*Session, *messages.SessionAccepted, map[string]json.RawMessage,
) {
	panic("not implemented: openPeer — transport, identity, auth elided")
}

func (*Session) Request(context.Context, *arcp.Envelope) (*arcp.Envelope, error) {
	panic("not implemented")
}
func (*Session) Send(context.Context, *arcp.Envelope) error  { panic("not implemented") }
func (*Session) Events(context.Context) <-chan arcp.Envelope { panic("not implemented") }
func (*Session) Close(context.Context) error                 { return nil }
