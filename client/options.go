// Package client implements the ARCP client surface: dial, submit,
// observe, cancel, subscribe.
package client

import (
	"log/slog"
	"time"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
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
