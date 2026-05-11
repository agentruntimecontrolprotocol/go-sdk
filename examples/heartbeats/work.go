// Worker work. Real version: a queue-of-work runner per role,
// invoking domain-specific code (indexer, extractor, archiver).
package main

import (
	"context"
	"encoding/json"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
	"github.com/agentruntimecontrolprotocol/go-sdk/messages"
)

func doWork(context.Context, map[string]any) (map[string]any, error) {
	panic("not implemented: doWork")
}

func marshalCompleted(result map[string]any) *messages.JobCompleted {
	b, _ := json.Marshal(result)
	return &messages.JobCompleted{Value: b}
}

// Session: as-if-published high-level wrapper.
type Session struct{}

func openSupervisor(context.Context) *Session { panic("not implemented: openSupervisor") }
func openWorker(context.Context) *Session     { panic("not implemented: openWorker") }
func (*Session) Request(context.Context, *arcp.Envelope) (*arcp.Envelope, error) {
	panic("not implemented")
}
func (*Session) Send(context.Context, *arcp.Envelope) error  { panic("not implemented") }
func (*Session) Events(context.Context) <-chan arcp.Envelope { panic("not implemented") }
func (*Session) Close(context.Context) error                 { return nil }
