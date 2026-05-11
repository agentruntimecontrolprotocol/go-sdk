# extensions

A custom domain (Software-Defined Radio) on top of ARCP via the
`arcpx.sdr.*.v1` extension namespace. Demonstrates §21 naming,
required-extension capability negotiation, and §21.3 unknown-message
handling for the optional experimental field.

## Before ARCP

Either bake SDR into the protocol (which is wrong — most agents
don't care) or invent a sibling protocol per domain (which forks
identity, transport, auth). Neither composes.

## With ARCP

```go
type SDRTune struct {
    CenterFreqHz  float64 `json:"center_freq_hz"`
    SampleRateHz  float64 `json:"sample_rate_hz"`
    PPMCorrection int     `json:"ppm_correction,omitempty"`
}
func (SDRTune) ARCPType() string { return "arcpx.sdr.tune.v1" }

c.Request(ctx, &arcp.Envelope{Payload: SDRTune{...}})

// optional, unadvertised — runtime ACKs (silent drop) per §21.3
c.Request(ctx, &arcp.Envelope{
    Extensions: map[string]json.RawMessage{"optional": json.RawMessage(`true`)},
    Payload:    ExperimentalDoppler{VelocityMPS: 7.4},
})
```

The IQ buffer is never inlined — capture returns an `artifact.ref`
and demodulate addresses it by ID.

## ARCP primitives

- `arcpx.<domain>.<name>.v<n>` extension naming — RFC §21.1.
- `capabilities.extensions` advertisement + required-set check — §7, §21.2.
- `extensions.optional: true` flag controls NACK vs silent-drop on
  unknown types — §21.3.
- Out-of-band binary via `artifact.ref` instead of inline base64 — §16.

## File tour

- `main.go` — extension type definitions + tune/gain/capture/demod flow.
- `sdr.go` — `ExperimentalDoppler` + `Session` shim.

## Variations

- Promote `arcpx.sdr.*.v1` to a stable namespace once it stabilizes
  — the wire bytes don't change, just the IANA-style registry status.
- Add `arcpx.sdr.iq.stream.v1` as a new `StreamKind` for live IQ
  flow rather than the capture/replay shape.
- Bridge to a hardware backend via [mcp](../mcp).
