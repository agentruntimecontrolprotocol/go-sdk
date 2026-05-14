// Package server hosts ARCP agents. Accept consumes a transport pair
// and runs one session over it; agents are registered via
// RegisterAgent / RegisterAgentVersion.
package server

import (
	"log/slog"
	"time"

	"github.com/agentruntimecontrolprotocol/go-sdk/auth"
	"github.com/agentruntimecontrolprotocol/go-sdk/internal/clock"
)

// Options configures a Server.
type Options struct {
	// Name is the runtime's advertised name in session.welcome.
	Name string
	// Version is the runtime's advertised version.
	Version string
	// HeartbeatInterval seeds heartbeat_interval_sec in welcome. Zero
	// disables heartbeats unless the client negotiates them; default
	// is 30s.
	HeartbeatInterval time.Duration
	// ResumeWindow seeds resume_window_sec; default 600s.
	ResumeWindow time.Duration
	// Verifier authenticates session.hello tokens. nil accepts no
	// tokens.
	Verifier auth.Verifier
	// Logger is the slog.Logger used by the runtime. nil uses
	// slog.Default().
	Logger *slog.Logger
	// Clock is the time source. nil uses clock.Real().
	Clock clock.Clock
	// AckLagThreshold is the number of unacked events that triggers a
	// back_pressure status emission. Zero disables.
	AckLagThreshold uint64
	// Features overrides the advertised feature list. Empty uses the
	// SDK default.
	Features []string
	// MaxResultBytes caps a single streamed result. Zero uses 32MiB.
	MaxResultBytes int64
	// ChunkSize caps an individual result_chunk body. Zero uses 1MiB.
	ChunkSize int64
}

func (o Options) withDefaults() Options {
	if o.Name == "" {
		o.Name = "arcp-go-runtime"
	}
	if o.Version == "" {
		o.Version = "1.0.0"
	}
	if o.HeartbeatInterval == 0 {
		o.HeartbeatInterval = 30 * time.Second
	}
	if o.ResumeWindow == 0 {
		o.ResumeWindow = 10 * time.Minute
	}
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	if o.Clock == nil {
		o.Clock = clock.Real()
	}
	if o.MaxResultBytes == 0 {
		o.MaxResultBytes = 32 << 20
	}
	if o.ChunkSize == 0 {
		o.ChunkSize = 1 << 20
	}
	return o
}
