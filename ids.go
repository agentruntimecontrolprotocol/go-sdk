package arcp

import "github.com/fizzpop/arcp-go/internal/ulid"

// SessionID identifies a single ARCP session (RFC §9). Format:
// "sess_" + ULID.
type SessionID string

// MessageID is the transport idempotency key carried in
// envelope.id (RFC §6.1.1, §6.4). Format: "msg_" + ULID.
type MessageID string

// JobID identifies a durable job (RFC §10). Format: "job_" + ULID.
type JobID string

// StreamID identifies an open stream (RFC §11). Format: "str_" + ULID.
type StreamID string

// SubscriptionID identifies an observer subscription (RFC §13).
// Format: "sub_" + ULID.
type SubscriptionID string

// TraceID is a stable id for one user-visible request or workflow
// (RFC §6.1.1, §17.1). Format: "trace_" + ULID.
type TraceID string

// SpanID identifies a single span within a trace tree (RFC §17.1).
// Format: "span_" + ULID.
type SpanID string

// ArtifactID identifies a stored artifact (RFC §16). Format:
// "art_" + ULID.
type ArtifactID string

// LeaseID identifies a granted permission lease (RFC §15.5). Format:
// "lease_" + ULID.
type LeaseID string

// CheckpointID identifies a job checkpoint (RFC §10.1). Format:
// "chk_" + ULID.
type CheckpointID string

// String returns the id as a plain string.
func (s SessionID) String() string      { return string(s) }
func (m MessageID) String() string      { return string(m) }
func (j JobID) String() string          { return string(j) }
func (s StreamID) String() string       { return string(s) }
func (s SubscriptionID) String() string { return string(s) }
func (t TraceID) String() string        { return string(t) }
func (s SpanID) String() string         { return string(s) }
func (a ArtifactID) String() string     { return string(a) }
func (l LeaseID) String() string        { return string(l) }
func (c CheckpointID) String() string   { return string(c) }

// NewSessionID generates a fresh session id.
func NewSessionID() SessionID { return SessionID("sess_" + ulid.New()) }

// NewMessageID generates a fresh message id.
func NewMessageID() MessageID { return MessageID("msg_" + ulid.New()) }

// NewJobID generates a fresh job id.
func NewJobID() JobID { return JobID("job_" + ulid.New()) }

// NewStreamID generates a fresh stream id.
func NewStreamID() StreamID { return StreamID("str_" + ulid.New()) }

// NewSubscriptionID generates a fresh subscription id.
func NewSubscriptionID() SubscriptionID { return SubscriptionID("sub_" + ulid.New()) }

// NewTraceID generates a fresh trace id.
func NewTraceID() TraceID { return TraceID("trace_" + ulid.New()) }

// NewSpanID generates a fresh span id.
func NewSpanID() SpanID { return SpanID("span_" + ulid.New()) }

// NewArtifactID generates a fresh artifact id.
func NewArtifactID() ArtifactID { return ArtifactID("art_" + ulid.New()) }

// NewLeaseID generates a fresh lease id.
func NewLeaseID() LeaseID { return LeaseID("lease_" + ulid.New()) }

// NewCheckpointID generates a fresh checkpoint id.
func NewCheckpointID() CheckpointID { return CheckpointID("chk_" + ulid.New()) }
