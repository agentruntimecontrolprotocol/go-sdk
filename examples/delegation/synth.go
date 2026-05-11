// Final-pass synthesizer. Real version: an Anthropic call that folds
// successful subagent outputs into prose, ignoring failed peers.
package main

import (
	"context"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
)

func synthesize(string, []Job) string {
	panic("not implemented: synthesize")
}

// Session: as-if-published high-level wrapper.
type Session struct{}

func openCoordinator(context.Context) *Session { panic("not implemented: openCoordinator") }
func (*Session) Request(context.Context, *arcp.Envelope) (*arcp.Envelope, error) {
	panic("not implemented")
}
func (*Session) Send(context.Context, *arcp.Envelope) error  { panic("not implemented") }
func (*Session) Events(context.Context) <-chan arcp.Envelope { panic("not implemented") }
func (*Session) Close(context.Context) error                 { return nil }
