// Package arcp implements the ARCP wire protocol: a transport-agnostic
// envelope for submitting, observing, and controlling long-running AI
// agent jobs.
//
// The root package exposes the on-the-wire envelope, sentinel error
// values, identifier helpers, and protocol-version constants. Higher
// level code lives in the sibling packages:
//
//   - [github.com/agentruntimecontrolprotocol/go-sdk/transport] —
//     pluggable bidirectional envelope channels (memory, WebSocket,
//     stdio).
//   - [github.com/agentruntimecontrolprotocol/go-sdk/messages] —
//     typed payload structs registered against the envelope.
//   - [github.com/agentruntimecontrolprotocol/go-sdk/client] — a
//     client that talks to a runtime.
//   - [github.com/agentruntimecontrolprotocol/go-sdk/server] — a
//     runtime that hosts agents.
//   - [github.com/agentruntimecontrolprotocol/go-sdk/auth] — bearer
//     verifier interface.
//   - [github.com/agentruntimecontrolprotocol/go-sdk/middleware] —
//     host-adapter sub-packages for net/http, chi, and OTel.
//
// The wire format is JSON; envelopes carry an [Envelope.Payload] as
// raw bytes so the read loop can hand off without parsing twice.
package arcp
