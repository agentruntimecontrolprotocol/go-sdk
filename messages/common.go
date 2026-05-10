package messages

import (
	"time"

	"github.com/fizzpop/arcp-go"
)

// AuthScheme names the credential format used in session.open /
// session.authenticate (RFC §8.2).
type AuthScheme string

// Defined auth schemes (RFC §8.2). v0.1 implements bearer, signed_jwt,
// and none. mtls and oauth2 are deferred.
const (
	AuthSchemeBearer    AuthScheme = "bearer"
	AuthSchemeMTLS      AuthScheme = "mtls"
	AuthSchemeOAuth2    AuthScheme = "oauth2"
	AuthSchemeSignedJWT AuthScheme = "signed_jwt"
	AuthSchemeNone      AuthScheme = "none"
)

// Auth carries credentials in session.open / session.authenticate
// (RFC §8.2).
type Auth struct {
	Scheme AuthScheme `json:"scheme"`
	Token  string     `json:"token,omitempty"`
}

// ClientIdentity describes the client kind, version, fingerprint, and
// principal advertised in session.open (RFC §8.2).
type ClientIdentity struct {
	Kind        string `json:"kind"`
	Version     string `json:"version"`
	Fingerprint string `json:"fingerprint,omitempty"`
	Principal   string `json:"principal,omitempty"`
}

// RuntimeIdentity describes the runtime identity advertised in
// session.accepted (RFC §8.3).
type RuntimeIdentity struct {
	Kind        string     `json:"kind"`
	Version     string     `json:"version"`
	Fingerprint string     `json:"fingerprint,omitempty"`
	TrustLevel  TrustLevel `json:"trust_level,omitempty"`
}

// TrustLevel is the trust classification of a session participant
// (RFC §15.3).
type TrustLevel string

// Defined trust levels (RFC §15.3).
const (
	TrustUntrusted   TrustLevel = "untrusted"
	TrustConstrained TrustLevel = "constrained"
	TrustTrusted     TrustLevel = "trusted"
	TrustPrivileged  TrustLevel = "privileged"
)

// Capabilities is the negotiated capability set for a session
// (RFC §7). Boolean fields default to false when absent.
type Capabilities struct {
	Streaming                       bool     `json:"streaming,omitempty"`
	DurableJobs                     bool     `json:"durable_jobs,omitempty"`
	Checkpoints                     bool     `json:"checkpoints,omitempty"`
	BinaryStreams                   bool     `json:"binary_streams,omitempty"`
	AgentHandoff                    bool     `json:"agent_handoff,omitempty"`
	HumanInput                      bool     `json:"human_input,omitempty"`
	Artifacts                       bool     `json:"artifacts,omitempty"`
	Subscriptions                   bool     `json:"subscriptions,omitempty"`
	ScheduledJobs                   bool     `json:"scheduled_jobs,omitempty"`
	Anonymous                       bool     `json:"anonymous,omitempty"`
	Interrupt                       bool     `json:"interrupt,omitempty"`
	HeartbeatIntervalSeconds        int      `json:"heartbeat_interval_seconds,omitempty"`
	HeartbeatRecovery               string   `json:"heartbeat_recovery,omitempty"` // "fail" | "block"
	BinaryEncoding                  []string `json:"binary_encoding,omitempty"`    // "base64" | "sidecar"
	ArtifactRetentionDefaultSeconds int      `json:"artifact_retention_default_seconds,omitempty"`
	ArtifactRetentionMaxSeconds     int      `json:"artifact_retention_max_seconds,omitempty"`
	Extensions                      []string `json:"extensions,omitempty"`
}

// Lease is the materialized form of a granted permission (RFC §15.5).
// Carried in session.accepted and lease.granted/extended payloads.
type Lease struct {
	LeaseID    arcp.LeaseID `json:"lease_id,omitempty"`
	Permission string       `json:"permission,omitempty"`
	Resource   string       `json:"resource,omitempty"`
	Operation  string       `json:"operation,omitempty"`
	ExpiresAt  time.Time    `json:"expires_at"`
}

// ErrorPayload is the structured error body used by tool.error,
// stream.error, job.failed, etc. (RFC §18.1).
type ErrorPayload struct {
	Code      arcp.ErrorCode `json:"code"`
	Message   string         `json:"message"`
	Retryable bool           `json:"retryable,omitempty"`
	Details   map[string]any `json:"details,omitempty"`
	TraceID   arcp.TraceID   `json:"trace_id,omitempty"`
}

// AsArcpError converts an ErrorPayload to an *arcp.Error.
func (e ErrorPayload) AsArcpError() *arcp.Error {
	return &arcp.Error{
		Code:      e.Code,
		Message:   e.Message,
		Retryable: e.Retryable,
		Details:   e.Details,
	}
}

// FromArcpError builds an ErrorPayload from an *arcp.Error.
func FromArcpError(err *arcp.Error) ErrorPayload {
	if err == nil {
		return ErrorPayload{Code: arcp.CodeOK}
	}
	return ErrorPayload{
		Code:      err.Code,
		Message:   err.Message,
		Retryable: err.Retryable,
		Details:   err.Details,
	}
}

// CancelTarget enumerates what a cancel/interrupt may address (§10.4).
type CancelTarget string

// Defined cancel targets.
const (
	CancelTargetJob     CancelTarget = "job"
	CancelTargetStream  CancelTarget = "stream"
	CancelTargetSession CancelTarget = "session"
)

// LogLevel is the severity of a log envelope (RFC §17.2).
type LogLevel string

// Defined log levels (RFC §17.2).
const (
	LogTrace    LogLevel = "trace"
	LogDebug    LogLevel = "debug"
	LogInfo     LogLevel = "info"
	LogWarn     LogLevel = "warn"
	LogError    LogLevel = "error"
	LogCritical LogLevel = "critical"
)

// register is a tiny helper to wire an init-time RegisterMessageType
// call. Keeps each messages/<group>.go init() compact.
func register(name string, factory func() arcp.MessageType) {
	arcp.RegisterMessageType(name, factory)
}
