package arcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
)

// Envelope is the wire-level frame for every ARCP message. Field
// names map exactly onto the JSON object documented in the spec.
type Envelope struct {
	// ARCP is the protocol version literal. Always "1" on the wire.
	ARCP string `json:"arcp"`
	// ID is a unique message identifier (UUIDv7 recommended).
	ID string `json:"id"`
	// Type is the wire-type token, for example "job.submit".
	Type string `json:"type"`
	// SessionID identifies the owning session; empty on session.hello.
	SessionID string `json:"session_id,omitempty"`
	// JobID is set on job-scoped envelopes (job.*).
	JobID string `json:"job_id,omitempty"`
	// TraceID is a 32-hex-char W3C trace identifier when present.
	TraceID string `json:"trace_id,omitempty"`
	// EventSeq is the session-scoped monotonic sequence for
	// job.event / job.result / job.error. Zero on every other type.
	EventSeq uint64 `json:"event_seq,omitempty"`
	// Payload is the typed payload, kept raw so the dispatch loop can
	// hand it off to a registered MessageType without parsing twice.
	Payload json.RawMessage `json:"payload,omitempty"`
	// Extensions carries x-vendor.* namespaced opaque fields per the
	// spec extensions namespace.
	Extensions map[string]json.RawMessage `json:"extensions,omitempty"`
}

// Validate reports a structured *Error if the envelope is missing
// fields the protocol mandates.
func (e *Envelope) Validate() error {
	if e.ARCP != ProtocolVersion {
		return ErrInvalidRequest.WithMessage(fmt.Sprintf("envelope arcp must be %q", ProtocolVersion))
	}
	if e.ID == "" {
		return ErrInvalidRequest.WithMessage("envelope id is required")
	}
	if e.Type == "" {
		return ErrInvalidRequest.WithMessage("envelope type is required")
	}
	return nil
}

// NewEnvelope returns an Envelope with arcp/id pre-populated.
func NewEnvelope(typ string, payload any) (Envelope, error) {
	body, err := MarshalPayload(payload)
	if err != nil {
		return Envelope{}, err
	}
	return Envelope{
		ARCP:    ProtocolVersion,
		ID:      NewEnvelopeID(),
		Type:    typ,
		Payload: body,
	}, nil
}

// MarshalPayload encodes any value to json.RawMessage. nil and
// already-raw values pass through.
func MarshalPayload(payload any) (json.RawMessage, error) {
	switch v := payload.(type) {
	case nil:
		return nil, nil
	case json.RawMessage:
		return v, nil
	case []byte:
		return json.RawMessage(v), nil
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, ErrInvalidRequest.WithMessage("payload marshal: " + err.Error())
	}
	return body, nil
}

// DecodePayload unmarshals e.Payload into v.
func (e *Envelope) DecodePayload(v any) error {
	if len(e.Payload) == 0 {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(e.Payload))
	if err := dec.Decode(v); err != nil {
		return ErrInvalidRequest.WithMessage("payload decode: " + err.Error())
	}
	return nil
}

// MessageType is implemented by every typed payload struct registered
// against the envelope dispatch table.
type MessageType interface {
	ARCPType() string
}

var (
	registryMu sync.RWMutex
	registry   = map[string]reflect.Type{}
)

// RegisterMessageType associates the concrete struct behind m with its
// wire-type string. Each wire-type string may only be registered once;
// duplicate registration panics. Call from init() in messages/*.go.
func RegisterMessageType(m MessageType) {
	t := m.ARCPType()
	if t == "" {
		panic("arcp: RegisterMessageType called with empty type")
	}
	registryMu.Lock()
	defer registryMu.Unlock()
	if existing, ok := registry[t]; ok {
		panic(fmt.Sprintf("arcp: duplicate MessageType registration for %q (existing: %v)", t, existing))
	}
	registry[t] = reflect.TypeOf(m).Elem()
}

// NewPayloadForType returns a zero value of the registered type for typ
// suitable for json.Unmarshal. It returns nil if typ is not registered.
func NewPayloadForType(typ string) any {
	registryMu.RLock()
	defer registryMu.RUnlock()
	t, ok := registry[typ]
	if !ok {
		return nil
	}
	return reflect.New(t).Interface()
}

// RegisteredTypes returns the names of every registered MessageType,
// useful for diagnostics.
func RegisteredTypes() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	out := make([]string, 0, len(registry))
	for k := range registry {
		out = append(out, k)
	}
	return out
}
