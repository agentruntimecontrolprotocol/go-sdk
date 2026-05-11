// Sink stand-ins. Real versions:
//   - StdoutSink: log/slog summarizer.
//   - SQLiteSink: arcp/store/eventlog schema for replay.
//   - OTLPSink: forwards metric + trace.span via otelhttp.
package main

import (
	"context"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
)

type StdoutSink struct{}
type SQLiteSink struct{ path string }
type OTLPSink struct{ endpoint string }

func (StdoutSink) Handle(context.Context, arcp.Envelope) error {
	panic("not implemented: StdoutSink.Handle")
}

func (*SQLiteSink) Handle(context.Context, arcp.Envelope) error {
	panic("not implemented: SQLiteSink.Handle")
}

func (*SQLiteSink) Close() error { return nil }

func (*OTLPSink) Handle(context.Context, arcp.Envelope) error {
	panic("not implemented: OTLPSink.Handle")
}

func newSinks() (StdoutSink, *SQLiteSink, *OTLPSink) {
	return StdoutSink{},
		&SQLiteSink{path: "replay.sqlite"},
		&OTLPSink{endpoint: "..."}
}

// Session is the as-if-published high-level client wrapper. Real
// shape would extend client.Client with a pending-request registry,
// an Events() fan-out channel, and an Envelope helper that fills in
// session_id and timestamps. Setup elided.
type Session struct{}

func openObserver(context.Context) *Session {
	panic("not implemented: openObserver — transport, identity, auth elided")
}

func (*Session) Request(context.Context, *arcp.Envelope) (*arcp.Envelope, error) {
	panic("not implemented: Session.Request")
}

func (*Session) Send(context.Context, *arcp.Envelope) error {
	panic("not implemented: Session.Send")
}

func (*Session) Events(context.Context) <-chan arcp.Envelope {
	panic("not implemented: Session.Events")
}

func (*Session) SessionID() arcp.SessionID {
	panic("not implemented: Session.SessionID")
}

func (*Session) Close(context.Context) error { return nil }
