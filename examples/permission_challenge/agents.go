// Generator + reviewer stand-ins. Real version: anthropic-sdk-go
// agents wired through a tiny tool-loop runner.
package main

import (
	"context"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
)

type Patch struct{ Diff string }

type ReviewVerdict struct {
	Grant  bool
	Reason string
}

func propose(context.Context, string, string) Patch {
	panic("not implemented: propose")
}

// review parses the patch out of request.payload.Resource (or looks
// it up by fingerprint), then runs the LLM.
func review(context.Context, string, arcp.Envelope) ReviewVerdict {
	panic("not implemented: review")
}

// Session: as-if-published high-level wrapper.
type Session struct{}

func openGenerator(context.Context) *Session { panic("not implemented: openGenerator") }
func openReviewer(context.Context) *Session  { panic("not implemented: openReviewer") }
func (*Session) Request(context.Context, *arcp.Envelope) (*arcp.Envelope, error) {
	panic("not implemented")
}
func (*Session) Send(context.Context, *arcp.Envelope) error  { panic("not implemented") }
func (*Session) Events(context.Context) <-chan arcp.Envelope { panic("not implemented") }
func (*Session) Close(context.Context) error                 { return nil }
