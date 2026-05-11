// Per-destination channel adapters. Real versions wrap ntfy.sh, SES,
// and the Slack web API. Each returns a value matching the request's
// response_schema.
package main

import (
	"context"
	"encoding/json"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
)

type ChannelAsk func(ctx context.Context, prompt string, schema json.RawMessage) (json.RawMessage, error)

func ntfyPhone(context.Context, string, json.RawMessage) (json.RawMessage, error) {
	panic("not implemented: ntfyPhone")
}
func emailOncall(context.Context, string, json.RawMessage) (json.RawMessage, error) {
	panic("not implemented: emailOncall")
}
func slackOps(context.Context, string, json.RawMessage) (json.RawMessage, error) {
	panic("not implemented: slackOps")
}

var registry = map[string]ChannelAsk{
	"ntfy:phone":   ntfyPhone,
	"email:oncall": emailOncall,
	"slack:ops":    slackOps,
}

func jsonExtension(key string, v any) map[string]json.RawMessage {
	b, _ := json.Marshal(v)
	return map[string]json.RawMessage{key: b}
}

// Session: as-if-published high-level wrapper.
type Session struct{}

func openHITL(context.Context) *Session                      { panic("not implemented: openHITL") }
func (*Session) Send(context.Context, *arcp.Envelope) error  { panic("not implemented") }
func (*Session) Events(context.Context) <-chan arcp.Envelope { panic("not implemented") }
func (*Session) Close(context.Context) error                 { return nil }
