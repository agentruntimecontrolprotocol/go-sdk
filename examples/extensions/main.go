// SDR domain via custom arcpx.sdr.*.v1 extension messages.
//
// Tune to 145.500 MHz (2 m FM calling), capture 5 s of IQ at 2.048 MS/s,
// NBFM-demodulate to 48 kHz PCM. Exercises §21 naming, capability
// advertisement, and unknown-message handling.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"slices"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
)

const (
	extTune       = "arcpx.sdr.tune.v1"
	extGain       = "arcpx.sdr.gain.v1"
	extCapture    = "arcpx.sdr.capture.v1"
	extDemodulate = "arcpx.sdr.demodulate.v1"
)

var allExtensions = []string{extTune, extGain, extCapture, extDemodulate}

// Extension payload structs. Per §21.1 the wire type is the namespaced
// name; the SDK's extension registry routes to these on decode.

type SDRTune struct {
	CenterFreqHz  float64 `json:"center_freq_hz"`
	SampleRateHz  float64 `json:"sample_rate_hz"`
	PPMCorrection int     `json:"ppm_correction,omitempty"`
}

func (SDRTune) ARCPType() string { return extTune }

type SDRGainStage struct {
	Name    string  `json:"name"`
	ValueDB float64 `json:"value_db"`
}

type SDRGain struct {
	Stages []SDRGainStage `json:"stages"`
}

func (SDRGain) ARCPType() string { return extGain }

type SDRCapture struct {
	Seconds       float64 `json:"seconds"`
	CaptureHandle string  `json:"capture_handle"`
	Decimate      int     `json:"decimate,omitempty"`
}

func (SDRCapture) ARCPType() string { return extCapture }

type SDRDemodulate struct {
	IQArtifactID arcp.ArtifactID `json:"iq_artifact_id"`
	Mode         string          `json:"mode"`
	AudioRateHz  int             `json:"audio_rate_hz"`
}

func (SDRDemodulate) ARCPType() string { return extDemodulate }

func main() {
	ctx := context.Background()
	c, accepted := openSDR(ctx) // capabilities.extensions=allExtensions on open
	defer c.Close(ctx)

	// If the runtime didn't advertise our required extension set,
	// refuse the session — RFC §7 / §21.2.
	for _, name := range allExtensions {
		if !slices.Contains(accepted.Capabilities.Extensions, name) {
			log.Fatal(arcp.NewError(arcp.CodeUnimplemented,
				fmt.Sprintf("runtime missing SDR extension: %s", name)))
		}
	}

	handle := arcp.NewMessageID()[len("msg_"):]

	if _, err := c.Request(ctx, &arcp.Envelope{
		Payload: SDRTune{
			CenterFreqHz: 145_500_000.0, SampleRateHz: 2_048_000.0,
			PPMCorrection: 1,
		}}); err != nil {
		log.Fatal(err)
	}
	if _, err := c.Request(ctx, &arcp.Envelope{
		Payload: SDRGain{Stages: []SDRGainStage{{Name: "TUNER", ValueDB: 28.0}}},
	}); err != nil {
		log.Fatal(err)
	}

	// Capture returns an artifact.ref pointing at the IQ buffer.
	// The buffer never travels inline — demodulate references it.
	cap, err := c.Request(ctx, &arcp.Envelope{
		Payload: SDRCapture{Seconds: 5.0, CaptureHandle: string(handle), Decimate: 1},
	})
	if err != nil {
		log.Fatal(err)
	}
	iqArt := captureArtifactID(cap)
	fmt.Printf("captured IQ -> %s\n", iqArt)

	audio, err := c.Request(ctx, &arcp.Envelope{
		Payload: SDRDemodulate{
			IQArtifactID: iqArt, Mode: "NBFM", AudioRateHz: 48_000,
		}})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("demod  PCM -> %s\n", captureArtifactID(audio))

	// §21.3 demonstration: unadvertised extension marked optional.
	// Runtime SHOULD ack (silent drop) rather than nack.
	optional, err := c.Request(ctx, &arcp.Envelope{
		Extensions: map[string]json.RawMessage{
			"optional": json.RawMessage(`true`),
		},
		Payload: ExperimentalDoppler{VelocityMPS: 7.4},
	})
	if err != nil {
		log.Print("optional unknown:", err)
		return
	}
	fmt.Printf("optional unknown -> %s\n", optional.Type())
}
