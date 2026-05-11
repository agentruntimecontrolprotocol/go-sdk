// Step bodies. Real version: a per-step function (Anthropic call for
// plan / synthesize / critique / finalize, vector retrieval for gather).
package main

import (
	"context"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
)

func runStep(context.Context, *Session, arcp.JobID, string, map[string]any) (any, error) {
	panic("not implemented: runStep")
}

// Session: as-if-published high-level wrapper.
type Session struct{}

func openClient(context.Context) *Session                    { panic("not implemented: openClient") }
func (*Session) Send(context.Context, *arcp.Envelope) error  { panic("not implemented") }
func (*Session) Events(context.Context) <-chan arcp.Envelope { panic("not implemented") }
func (*Session) Close(context.Context) error                 { return nil }
