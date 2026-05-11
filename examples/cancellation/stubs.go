// Setup elision. See subscriptions/sinks.go for rationale.
package main

import (
	"context"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
)

// Session: as-if-published high-level wrapper.
type Session struct{}

func openClient(context.Context) *Session { panic("not implemented: openClient") }
func (*Session) Request(context.Context, *arcp.Envelope) (*arcp.Envelope, error) {
	panic("not implemented")
}
func (*Session) Send(context.Context, *arcp.Envelope) error  { panic("not implemented") }
func (*Session) Events(context.Context) <-chan arcp.Envelope { panic("not implemented") }
func (*Session) Close(context.Context) error                 { return nil }
