package arcp

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// MessageType is implemented by every typed payload struct in the
// messages/ package. The single method returns the wire type string
// (RFC §6.2). Decoding routes on this string via the registry built up
// by RegisterMessageType.
type MessageType interface {
	ARCPType() string
}

// Priority is the QoS level of an envelope (RFC §6.5).
type Priority string

// Defined priority values (RFC §6.5).
const (
	PriorityLow      Priority = "low"
	PriorityNormal   Priority = "normal"
	PriorityHigh     Priority = "high"
	PriorityCritical Priority = "critical"
)

// Envelope is the canonical ARCP message container (RFC §6.1).
//
// All cross-the-wire ARCP messages travel as an Envelope. The Payload
// field holds the typed payload — its concrete Go type depends on the
// wire type field. Use Envelope.MarshalJSON / UnmarshalJSON to encode
// and decode; do not write the raw Envelope through encoding/json
// without going through these methods, because they manage the
// type/payload discriminator.
type Envelope struct {
	// ARCP is the protocol version understood by the sender (§6.1.1).
	// Empty on construction; set to ProtocolVersion at marshal time.
	ARCP string `json:"arcp"`
	// ID is the globally unique message id and transport idempotency
	// key (§6.1.1, §6.4).
	ID MessageID `json:"id"`
	// Timestamp is the sender timestamp in RFC 3339 format (§6.1.1).
	Timestamp time.Time `json:"timestamp"`
	// Source is the logical sender id (§6.1.1).
	Source string `json:"source,omitempty"`
	// Target is the logical recipient id (§6.1.1).
	Target string `json:"target,omitempty"`
	// SessionID is required once a session exists (§6.1.1).
	SessionID SessionID `json:"session_id,omitempty"`
	// JobID is required for durable job events (§6.1.1).
	JobID JobID `json:"job_id,omitempty"`
	// StreamID is required for stream events (§6.1.1).
	StreamID StreamID `json:"stream_id,omitempty"`
	// SubscriptionID is required for subscription delivery (§6.1.1).
	SubscriptionID SubscriptionID `json:"subscription_id,omitempty"`
	// TraceID is the stable id for one user-visible workflow (§6.1.1).
	TraceID TraceID `json:"trace_id,omitempty"`
	// SpanID is the span id for the current operation (§6.1.1).
	SpanID SpanID `json:"span_id,omitempty"`
	// ParentSpanID is the parent span id when the message is part of a
	// trace tree (§6.1.1).
	ParentSpanID SpanID `json:"parent_span_id,omitempty"`
	// CorrelationID is the id of the command or request this message
	// answers (§6.1.1).
	CorrelationID MessageID `json:"correlation_id,omitempty"`
	// CausationID is the id of the message that directly caused this
	// message (§6.1.1).
	CausationID MessageID `json:"causation_id,omitempty"`
	// IdempotencyKey is the logical idempotency key for the command
	// intent, distinct from ID (§6.1.1, §6.4).
	IdempotencyKey string `json:"idempotency_key,omitempty"`
	// Priority is the QoS level (§6.5). Default normal.
	Priority Priority `json:"priority,omitempty"`
	// Extensions is an object of namespaced extension fields (§21).
	Extensions map[string]json.RawMessage `json:"extensions,omitempty"`
	// Payload is the typed payload. Its concrete Go type matches the
	// wire `type` field.
	Payload MessageType `json:"-"`
}

// envelopeWire is the on-wire shape of an Envelope. It mirrors
// Envelope but stores Payload as a raw JSON message, which lets us
// dispatch to a typed payload struct based on Type during decode and
// re-encode without an extra round-trip during encode.
type envelopeWire struct {
	ARCP           string                     `json:"arcp"`
	ID             MessageID                  `json:"id"`
	Type           string                     `json:"type"`
	Timestamp      time.Time                  `json:"timestamp"`
	Source         string                     `json:"source,omitempty"`
	Target         string                     `json:"target,omitempty"`
	SessionID      SessionID                  `json:"session_id,omitempty"`
	JobID          JobID                      `json:"job_id,omitempty"`
	StreamID       StreamID                   `json:"stream_id,omitempty"`
	SubscriptionID SubscriptionID             `json:"subscription_id,omitempty"`
	TraceID        TraceID                    `json:"trace_id,omitempty"`
	SpanID         SpanID                     `json:"span_id,omitempty"`
	ParentSpanID   SpanID                     `json:"parent_span_id,omitempty"`
	CorrelationID  MessageID                  `json:"correlation_id,omitempty"`
	CausationID    MessageID                  `json:"causation_id,omitempty"`
	IdempotencyKey string                     `json:"idempotency_key,omitempty"`
	Priority       Priority                   `json:"priority,omitempty"`
	Extensions     map[string]json.RawMessage `json:"extensions,omitempty"`
	Payload        json.RawMessage            `json:"payload"`
}

// Type returns the wire type string of the envelope's payload, or the
// empty string if no payload is set.
func (e Envelope) Type() string {
	if e.Payload == nil {
		return ""
	}
	return e.Payload.ARCPType()
}

// MarshalJSON serializes an Envelope to ARCP wire format. The `arcp`
// version field is always set to ProtocolVersion. The `type` field is
// derived from Payload.ARCPType().
func (e Envelope) MarshalJSON() ([]byte, error) {
	if e.Payload == nil {
		return nil, NewError(CodeInvalidArgument, "envelope has nil payload")
	}
	payloadBytes, err := json.Marshal(e.Payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}
	wire := envelopeWire{
		ARCP:           ProtocolVersion,
		ID:             e.ID,
		Type:           e.Payload.ARCPType(),
		Timestamp:      e.Timestamp,
		Source:         e.Source,
		Target:         e.Target,
		SessionID:      e.SessionID,
		JobID:          e.JobID,
		StreamID:       e.StreamID,
		SubscriptionID: e.SubscriptionID,
		TraceID:        e.TraceID,
		SpanID:         e.SpanID,
		ParentSpanID:   e.ParentSpanID,
		CorrelationID:  e.CorrelationID,
		CausationID:    e.CausationID,
		IdempotencyKey: e.IdempotencyKey,
		Priority:       e.Priority,
		Extensions:     e.Extensions,
		Payload:        payloadBytes,
	}
	return json.Marshal(wire)
}

// UnmarshalJSON parses an Envelope from ARCP wire format. The wire
// `type` field is used to look up a registered factory from the
// dispatch registry; the payload is then decoded into a fresh value of
// the registered type.
//
// Unknown core types and unknown extension types are not handled here:
// this method returns *Error with code UNIMPLEMENTED so the runtime
// layer can apply RFC §21.3's policy (NACK or silent drop). Callers
// that want to peek at the type without decoding the payload should
// use UnmarshalEnvelopeHeader.
func (e *Envelope) UnmarshalJSON(data []byte) error {
	var wire envelopeWire
	if err := json.Unmarshal(data, &wire); err != nil {
		return fmt.Errorf("unmarshal envelope: %w", err)
	}
	factory, ok := lookupMessageFactory(wire.Type)
	if !ok {
		return NewError(CodeUnimplemented,
			fmt.Sprintf("unknown message type %q", wire.Type))
	}
	payload := factory()
	if len(wire.Payload) > 0 {
		if err := json.Unmarshal(wire.Payload, payload); err != nil {
			return NewError(CodeInvalidArgument,
				fmt.Sprintf("decode payload for %q: %v", wire.Type, err)).WithCause(err)
		}
	}
	*e = Envelope{
		ARCP:           wire.ARCP,
		ID:             wire.ID,
		Timestamp:      wire.Timestamp,
		Source:         wire.Source,
		Target:         wire.Target,
		SessionID:      wire.SessionID,
		JobID:          wire.JobID,
		StreamID:       wire.StreamID,
		SubscriptionID: wire.SubscriptionID,
		TraceID:        wire.TraceID,
		SpanID:         wire.SpanID,
		ParentSpanID:   wire.ParentSpanID,
		CorrelationID:  wire.CorrelationID,
		CausationID:    wire.CausationID,
		IdempotencyKey: wire.IdempotencyKey,
		Priority:       wire.Priority,
		Extensions:     wire.Extensions,
		Payload:        payload,
	}
	return nil
}

// EnvelopeHeader captures the routing fields of an envelope without
// decoding its payload. Useful when the receiver wants to inspect the
// type for permission checks or for the §21.3 unknown-message policy
// before incurring decode cost.
type EnvelopeHeader struct {
	ARCP           string         `json:"arcp"`
	ID             MessageID      `json:"id"`
	Type           string         `json:"type"`
	Timestamp      time.Time      `json:"timestamp"`
	SessionID      SessionID      `json:"session_id,omitempty"`
	JobID          JobID          `json:"job_id,omitempty"`
	StreamID       StreamID       `json:"stream_id,omitempty"`
	SubscriptionID SubscriptionID `json:"subscription_id,omitempty"`
	TraceID        TraceID        `json:"trace_id,omitempty"`
	CorrelationID  MessageID      `json:"correlation_id,omitempty"`
	CausationID    MessageID      `json:"causation_id,omitempty"`
	IdempotencyKey string         `json:"idempotency_key,omitempty"`
	Priority       Priority       `json:"priority,omitempty"`
	Optional       bool           `json:"-"`
}

// UnmarshalEnvelopeHeader parses only the envelope routing fields plus
// the extensions.optional flag (RFC §21.3). The payload is left
// undecoded.
func UnmarshalEnvelopeHeader(data []byte) (EnvelopeHeader, error) {
	var raw struct {
		ARCP           string                     `json:"arcp"`
		ID             MessageID                  `json:"id"`
		Type           string                     `json:"type"`
		Timestamp      time.Time                  `json:"timestamp"`
		SessionID      SessionID                  `json:"session_id,omitempty"`
		JobID          JobID                      `json:"job_id,omitempty"`
		StreamID       StreamID                   `json:"stream_id,omitempty"`
		SubscriptionID SubscriptionID             `json:"subscription_id,omitempty"`
		TraceID        TraceID                    `json:"trace_id,omitempty"`
		CorrelationID  MessageID                  `json:"correlation_id,omitempty"`
		CausationID    MessageID                  `json:"causation_id,omitempty"`
		IdempotencyKey string                     `json:"idempotency_key,omitempty"`
		Priority       Priority                   `json:"priority,omitempty"`
		Extensions     map[string]json.RawMessage `json:"extensions,omitempty"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return EnvelopeHeader{}, fmt.Errorf("unmarshal envelope header: %w", err)
	}
	hdr := EnvelopeHeader{
		ARCP:           raw.ARCP,
		ID:             raw.ID,
		Type:           raw.Type,
		Timestamp:      raw.Timestamp,
		SessionID:      raw.SessionID,
		JobID:          raw.JobID,
		StreamID:       raw.StreamID,
		SubscriptionID: raw.SubscriptionID,
		TraceID:        raw.TraceID,
		CorrelationID:  raw.CorrelationID,
		CausationID:    raw.CausationID,
		IdempotencyKey: raw.IdempotencyKey,
		Priority:       raw.Priority,
	}
	if opt, ok := raw.Extensions["optional"]; ok {
		var b bool
		if err := json.Unmarshal(opt, &b); err == nil {
			hdr.Optional = b
		}
	}
	return hdr, nil
}

// Message factory registry. Built up by init() functions in the
// messages/ subpackages — the only init() use permitted by the design
// rules.
var (
	messageRegistryMu sync.RWMutex
	messageRegistry   = map[string]func() MessageType{}
)

// RegisterMessageType records a payload constructor for a wire type
// name. Each messages/<group>.go init() function calls this once per
// message type. Calling RegisterMessageType twice for the same name
// panics — duplicate registration is an unrecoverable bug, not a
// runtime error.
func RegisterMessageType(name string, factory func() MessageType) {
	if name == "" {
		panic("arcp.RegisterMessageType: empty name")
	}
	if factory == nil {
		panic("arcp.RegisterMessageType: nil factory for " + name)
	}
	messageRegistryMu.Lock()
	defer messageRegistryMu.Unlock()
	if _, dup := messageRegistry[name]; dup {
		panic("arcp.RegisterMessageType: duplicate registration for " + name)
	}
	messageRegistry[name] = factory
}

// lookupMessageFactory returns the registered factory for a wire type,
// or false if none is registered.
func lookupMessageFactory(name string) (func() MessageType, bool) {
	messageRegistryMu.RLock()
	defer messageRegistryMu.RUnlock()
	f, ok := messageRegistry[name]
	return f, ok
}

// RegisteredMessageTypes returns the set of currently-registered wire
// type names. Useful for diagnostics; not for hot paths.
func RegisteredMessageTypes() []string {
	messageRegistryMu.RLock()
	defer messageRegistryMu.RUnlock()
	out := make([]string, 0, len(messageRegistry))
	for k := range messageRegistry {
		out = append(out, k)
	}
	sortStrings(out)
	return out
}
