// Stand-in for the Anthropic tool-use loop. Real version uses
// anthropic-sdk-go with a system prompt and yields one LLMStep per
// turn over a channel.
package main

import (
	"context"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
)

type ToolCall struct {
	Argv   []string
	Reason string
}

type LLMStep struct {
	Thought  string
	ToolCall *ToolCall
	Final    string
}

func llmLoop(context.Context, string) <-chan LLMStep {
	panic("not implemented: llmLoop")
}

// Session: as-if-published high-level wrapper. See subscriptions/sinks.go
// for the rationale.
type Session struct{}

func openConstrained(context.Context) *Session {
	panic("not implemented: openConstrained — transport, identity, auth elided")
}
func (*Session) Request(context.Context, *arcp.Envelope) (*arcp.Envelope, error) {
	panic("not implemented")
}
func (*Session) Send(context.Context, *arcp.Envelope) error { panic("not implemented") }
func (*Session) Close(context.Context) error                { return nil }
