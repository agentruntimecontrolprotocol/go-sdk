// SDR back-end stand-in. Real version wraps a vendored RTL-SDR /
// SoapySDR FFI binding; SDR is a niche so most Go projects shell out
// to `rtl_sdr` and consume IQ from a pipe.
package main

import (
	"context"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
	"github.com/agentruntimecontrolprotocol/go-sdk/messages"
)

// ExperimentalDoppler is the optional, unadvertised extension type
// used to exercise §21.3 unknown-message handling.
type ExperimentalDoppler struct {
	VelocityMPS float64 `json:"velocity_mps"`
}

func (ExperimentalDoppler) ARCPType() string { return "arcpx.sdr.experimental_doppler.v1" }

// captureArtifactID pulls the artifact_id out of an SDR capture or
// demod response. Real SDR runtime would return a typed payload.
func captureArtifactID(env *arcp.Envelope) arcp.ArtifactID {
	if ref, ok := env.Payload.(*messages.ArtifactRef); ok {
		return ref.ArtifactID
	}
	return ""
}

// Session: as-if-published high-level wrapper.
type Session struct{}

func openSDR(context.Context) (*Session, *messages.SessionAccepted) {
	panic("not implemented: openSDR — transport, identity, auth, capabilities elided")
}
func (*Session) Request(context.Context, *arcp.Envelope) (*arcp.Envelope, error) {
	panic("not implemented")
}
func (*Session) Close(context.Context) error { return nil }
