// Package arcp is the Go reference implementation of the Agent Runtime Control
// Protocol (ARCP) v1.0, defined in RFC-0001-v2.md (in this directory).
//
// ARCP is a transport-agnostic, schema-first protocol for secure, observable,
// streaming-native execution of tools, resources, workflows, and agent-to-agent
// interactions. This package contains the canonical envelope, typed
// identifiers, error model, extension registry, and trace context helpers.
//
// Subpackages provide the rest of the implementation:
//
//   - messages: Typed payloads for every protocol message (RFC §6.2).
//   - runtime:  Server-side runtime: sessions, jobs, streams, leases, etc.
//   - client:   Client-side helper for opening sessions and issuing commands.
//   - transport: Transport adapters (WebSocket, stdio, in-memory).
//   - store:    SQLite-backed event log with idempotency and replay.
//   - auth:     Authentication scheme implementations (bearer, signed_jwt).
//
// The reference RFC sections are referenced throughout godoc comments. The
// authoritative specification is RFC-0001-v2.md; if this implementation
// disagrees with the RFC, the RFC wins.
package arcp
