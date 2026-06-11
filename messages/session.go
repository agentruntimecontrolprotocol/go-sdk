package messages

import (
	"encoding/json"
	"time"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
)

// ClientInfo identifies the dialing client.
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

// RuntimeInfo identifies the responding runtime.
type RuntimeInfo struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

// AuthInfo carries handshake-time authentication. Bearer is the only
// scheme defined by the protocol; deployment policy may extend it.
type AuthInfo struct {
	Scheme string `json:"scheme"`
	Token  string `json:"token,omitempty"`
}

// AgentEntry is one entry in the agent inventory advertised in
// session.welcome.
type AgentEntry struct {
	Name     string   `json:"name"`
	Versions []string `json:"versions,omitempty"`
	Default  string   `json:"default,omitempty"`
}

// HelloCapabilities is the capability advertisement sent in
// session.hello.payload.capabilities.
type HelloCapabilities struct {
	Encodings []string `json:"encodings,omitempty"`
	Features  []string `json:"features,omitempty"`
}

// WelcomeCapabilities is the response capability set returned in
// session.welcome.payload.capabilities.
type WelcomeCapabilities struct {
	Encodings []string     `json:"encodings,omitempty"`
	Features  []string     `json:"features,omitempty"`
	Agents    []AgentEntry `json:"agents,omitempty"`
}

// SessionHello is the first envelope a client sends after dialing.
type SessionHello struct {
	Client       ClientInfo        `json:"client"`
	Auth         AuthInfo          `json:"auth"`
	Capabilities HelloCapabilities `json:"capabilities"`
	Resume       *ResumeRequest    `json:"resume,omitempty"`
}

// ARCPType returns the wire-type string for SessionHello.
func (*SessionHello) ARCPType() string { return TypeSessionHello }

// ResumeRequest is the optional resume block inside SessionHello.
type ResumeRequest struct {
	SessionID    string `json:"session_id"`
	ResumeToken  string `json:"resume_token"`
	LastEventSeq uint64 `json:"last_event_seq"`
}

// SessionWelcome is the runtime's response to session.hello.
type SessionWelcome struct {
	Runtime              RuntimeInfo         `json:"runtime"`
	ResumeToken          string              `json:"resume_token,omitempty"`
	ResumeWindowSec      int                 `json:"resume_window_sec,omitempty"`
	HeartbeatIntervalSec int                 `json:"heartbeat_interval_sec,omitempty"`
	Capabilities         WelcomeCapabilities `json:"capabilities"`
}

// ARCPType returns the wire-type string for SessionWelcome.
func (*SessionWelcome) ARCPType() string { return TypeSessionWelcome }

// SessionError is a session-scoped failure. Most uses are actually
// per-request rejections (an unknown agent on job.submit, a denied
// job.subscribe, an unknown job.cancel, …) that do not end the session.
// RequestID echoes the id of the offending request envelope and JobID
// echoes its job_id (when applicable) so clients can correlate the
// failure to the originating call instead of treating every error as
// session-fatal.
type SessionError struct {
	Code      arcp.ErrorCode `json:"code"`
	Message   string         `json:"message,omitempty"`
	Retryable bool           `json:"retryable"`
	Details   map[string]any `json:"details,omitempty"`
	RequestID string         `json:"request_id,omitempty"`
	JobID     string         `json:"job_id,omitempty"`
}

// ARCPType returns the wire-type string for SessionError.
func (*SessionError) ARCPType() string { return TypeSessionError }

// SessionBye signals a polite session close.
type SessionBye struct {
	Reason string `json:"reason,omitempty"`
}

// ARCPType returns the wire-type string for SessionBye.
func (*SessionBye) ARCPType() string { return TypeSessionBye }

// SessionPing is the heartbeat probe; the receiver must respond with
// SessionPong carrying PingNonce equal to Nonce.
type SessionPing struct {
	Nonce  string    `json:"nonce"`
	SentAt time.Time `json:"sent_at"`
}

// ARCPType returns the wire-type string for SessionPing.
func (*SessionPing) ARCPType() string { return TypeSessionPing }

// SessionPong responds to a SessionPing.
type SessionPong struct {
	PingNonce  string    `json:"ping_nonce"`
	ReceivedAt time.Time `json:"received_at"`
}

// ARCPType returns the wire-type string for SessionPong.
func (*SessionPong) ARCPType() string { return TypeSessionPong }

// SessionAck declares the client's highest processed event_seq.
type SessionAck struct {
	LastProcessedSeq uint64 `json:"last_processed_seq"`
}

// ARCPType returns the wire-type string for SessionAck.
func (*SessionAck) ARCPType() string { return TypeSessionAck }

// SessionListJobs is the read-only inventory request.
type SessionListJobs struct {
	Filter ListJobsFilter `json:"filter,omitempty"`
	Limit  int            `json:"limit,omitempty"`
	Cursor string         `json:"cursor,omitempty"`
}

// ARCPType returns the wire-type string for SessionListJobs.
func (*SessionListJobs) ARCPType() string { return TypeSessionListJobs }

// ListJobsFilter narrows the result set of session.list_jobs.
type ListJobsFilter struct {
	Status        []string   `json:"status,omitempty"`
	Agent         string     `json:"agent,omitempty"`
	CreatedAfter  *time.Time `json:"created_after,omitempty"`
	CreatedBefore *time.Time `json:"created_before,omitempty"`
}

// SessionJobs is the response to session.list_jobs.
type SessionJobs struct {
	RequestID  string    `json:"request_id,omitempty"`
	Jobs       []JobInfo `json:"jobs"`
	NextCursor string    `json:"next_cursor,omitempty"`
}

// ARCPType returns the wire-type string for SessionJobs.
func (*SessionJobs) ARCPType() string { return TypeSessionJobs }

// JobInfo describes one job in session.jobs / job.subscribed.
type JobInfo struct {
	JobID        string          `json:"job_id"`
	Agent        string          `json:"agent"`
	Status       string          `json:"status"`
	Lease        arcp.Lease      `json:"lease,omitempty"`
	ParentJobID  string          `json:"parent_job_id,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
	TraceID      string          `json:"trace_id,omitempty"`
	LastEventSeq uint64          `json:"last_event_seq,omitempty"`
	Extra        json.RawMessage `json:"-"`
}

func init() {
	arcp.RegisterMessageType(&SessionHello{})
	arcp.RegisterMessageType(&SessionWelcome{})
	arcp.RegisterMessageType(&SessionError{})
	arcp.RegisterMessageType(&SessionBye{})
	arcp.RegisterMessageType(&SessionPing{})
	arcp.RegisterMessageType(&SessionPong{})
	arcp.RegisterMessageType(&SessionAck{})
	arcp.RegisterMessageType(&SessionListJobs{})
	arcp.RegisterMessageType(&SessionJobs{})
}
