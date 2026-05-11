// Primary + critic LLM stand-ins.
package main

import (
	"context"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
)

// primaryStep performs one reasoning step. Real version: an Anthropic
// call that folds the prior critique into the prompt when present.
func primaryStep(context.Context, string, *Critique) string {
	panic("not implemented: primaryStep")
}

// critiqueThought returns (severity, summary, suggestion, consumed_tokens).
// Severity is one of "nudge" | "warn" | "halt".
func critiqueThought(context.Context, string) (string, string, string, int) {
	panic("not implemented: critiqueThought")
}

// Session: as-if-published high-level wrapper.
type Session struct{}

func openPrimary(context.Context) *Session { panic("not implemented: openPrimary") }
func openMirror(context.Context) *Session  { panic("not implemented: openMirror") }
func (*Session) Request(context.Context, *arcp.Envelope) (*arcp.Envelope, error) {
	panic("not implemented")
}
func (*Session) Send(context.Context, *arcp.Envelope) error  { panic("not implemented") }
func (*Session) Events(context.Context) <-chan arcp.Envelope { panic("not implemented") }
func (*Session) SessionID() arcp.SessionID                   { panic("not implemented") }
func (*Session) Close(context.Context) error                 { return nil }
