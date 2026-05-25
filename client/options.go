// Package client implements the ARCP client surface: dial, submit,
// observe, cancel, subscribe.
package client

import (
	"log/slog"
	"time"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
	"github.com/agentruntimecontrolprotocol/go-sdk/messages"
)

// Options configures a Client.
type Options struct {
	// ClientName advertised in session.hello.client.name.
	ClientName string
	// ClientVersion advertised in session.hello.client.version.
	ClientVersion string
	// Token is the bearer token passed via session.hello.auth.token.
	Token string
	// Features overrides the advertised feature list. Empty uses the
	// SDK default set.
	Features []string
	// Logger receives client diagnostics. nil uses slog.Default().
	Logger *slog.Logger
	// AutoAck coalesces session.ack emission to one every Window
	// events processed (zero disables auto-ack).
	AutoAckWindow uint64
	// AutoAckInterval bounds how long auto-ack waits between sends.
	AutoAckInterval time.Duration
	// Resume, if non-nil, asks the runtime to continue a previously
	// dropped session: the SessionID and ResumeToken come from the
	// prior welcome, and LastEventSeq is the highest event_seq the
	// caller has already processed. The runtime replays every event
	// with seq greater than LastEventSeq before live traffic resumes.
	// The token is single-use; the next welcome carries a fresh one.
	Resume *messages.ResumeRequest
	// EventDeliveryTimeout bounds how long the dispatcher will block
	// trying to deliver a single envelope to a slow JobHandle or
	// Subscription consumer before closing it with an overflow error.
	// Zero means block indefinitely (the lossless default — the
	// consumer is expected to drain). Set this for back-pressure
	// sensitive callers that must not stall the read loop.
	EventDeliveryTimeout time.Duration
}

func (o Options) withDefaults() Options {
	if o.ClientName == "" {
		o.ClientName = "arcp-go-client"
	}
	if o.ClientVersion == "" {
		o.ClientVersion = arcp.SDKVersion
	}
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	if len(o.Features) == 0 {
		f := make([]string, len(arcp.Features))
		copy(f, arcp.Features)
		o.Features = f
	}
	if o.AutoAckInterval == 0 {
		o.AutoAckInterval = 250 * time.Millisecond
	}
	return o
}
