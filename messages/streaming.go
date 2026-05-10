package messages

import (
	"encoding/json"

	"github.com/fizzpop/arcp-go"
)

// Wire type names for the streaming group (RFC §6.2, §11).
const (
	TypeStreamOpen  = "stream.open"
	TypeStreamChunk = "stream.chunk"
	TypeStreamClose = "stream.close"
	TypeStreamError = "stream.error"
)

// StreamKind enumerates the defined stream kinds (RFC §11.1).
type StreamKind string

// Defined stream kinds (RFC §11.1).
const (
	StreamKindText    StreamKind = "text"
	StreamKindBinary  StreamKind = "binary"
	StreamKindEvent   StreamKind = "event"
	StreamKindLog     StreamKind = "log"
	StreamKindMetric  StreamKind = "metric"
	StreamKindThought StreamKind = "thought"
)

// StreamOpen declares a new stream (RFC §11.1).
type StreamOpen struct {
	Kind        StreamKind `json:"kind"`
	ContentType string     `json:"content_type,omitempty"`
	Encoding    string     `json:"encoding,omitempty"`
}

// ARCPType returns the wire type name.
func (StreamOpen) ARCPType() string { return TypeStreamOpen }

// StreamChunk is a single chunk on an open stream (RFC §11.1, §11.4).
//
// The shape of a chunk varies by stream kind. v0.1 uses a permissive
// payload that captures the common fields; consumers should switch on
// the parent stream's Kind (cached at the runtime/client) and decode
// the relevant fields. For binary streams (RFC §11.3 first bullet),
// `Data` carries base64-encoded bytes.
type StreamChunk struct {
	Sequence    int             `json:"sequence"`
	Data        string          `json:"data,omitempty"`       // base64 for binary, raw text for text
	JSON        json.RawMessage `json:"json,omitempty"`       // for kind=event
	Role        string          `json:"role,omitempty"`       // for kind=thought
	Content     string          `json:"content,omitempty"`    // for kind=thought / log
	Redacted    bool            `json:"redacted,omitempty"`   // for kind=thought
	Level       LogLevel        `json:"level,omitempty"`      // for kind=log
	Attributes  map[string]any  `json:"attributes,omitempty"` // for kind=log
	Name        string          `json:"name,omitempty"`       // for kind=metric
	Value       float64         `json:"value,omitempty"`      // for kind=metric
	Unit        string          `json:"unit,omitempty"`       // for kind=metric
	ContentType string          `json:"content_type,omitempty"`
	SHA256      string          `json:"sha256,omitempty"`
}

// ARCPType returns the wire type name.
func (StreamChunk) ARCPType() string { return TypeStreamChunk }

// StreamClose ends a stream cleanly (RFC §11).
type StreamClose struct {
	Reason string `json:"reason,omitempty"`
}

// ARCPType returns the wire type name.
func (StreamClose) ARCPType() string { return TypeStreamClose }

// StreamError ends a stream with an error (RFC §11, §10.4).
type StreamError struct {
	ErrorPayload
}

// ARCPType returns the wire type name.
func (StreamError) ARCPType() string { return TypeStreamError }

func init() {
	register(TypeStreamOpen, func() arcp.MessageType { return &StreamOpen{} })
	register(TypeStreamChunk, func() arcp.MessageType { return &StreamChunk{} })
	register(TypeStreamClose, func() arcp.MessageType { return &StreamClose{} })
	register(TypeStreamError, func() arcp.MessageType { return &StreamError{} })
}
