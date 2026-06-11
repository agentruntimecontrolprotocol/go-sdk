package messages

import (
	"encoding/json"
	"time"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
)

// LeaseConstraints carries optional runtime constraints attached to a
// lease, currently the expires_at deadline.
type LeaseConstraints struct {
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// JobSubmit is the client request to create a new job.
type JobSubmit struct {
	Agent            string            `json:"agent"`
	Input            json.RawMessage   `json:"input,omitempty"`
	LeaseRequest     arcp.Lease        `json:"lease_request,omitempty"`
	LeaseConstraints *LeaseConstraints `json:"lease_constraints,omitempty"`
	IdempotencyKey   string            `json:"idempotency_key,omitempty"`
	MaxRuntimeSec    int               `json:"max_runtime_sec,omitempty"`
}

// ARCPType returns the wire-type string for JobSubmit.
func (*JobSubmit) ARCPType() string { return TypeJobSubmit }

// JobAccepted echoes the effective lease and constraints back to the
// submitter.
type JobAccepted struct {
	JobID            string                    `json:"job_id"`
	Lease            arcp.Lease                `json:"lease,omitempty"`
	LeaseConstraints *LeaseConstraints         `json:"lease_constraints,omitempty"`
	Budget           map[arcp.Currency]float64 `json:"budget,omitempty"`
	Credentials      []Credential              `json:"credentials,omitempty"`
	AcceptedAt       time.Time                 `json:"accepted_at"`
	TraceID          string                    `json:"trace_id,omitempty"`
	ParentJobID      string                    `json:"parent_job_id,omitempty"`
	Agent            string                    `json:"agent,omitempty"`
}

// ARCPType returns the wire-type string for JobAccepted.
func (*JobAccepted) ARCPType() string { return TypeJobAccepted }

// CredentialConstraints describes the lease-derived limits baked into
// a provisioned credential.
type CredentialConstraints struct {
	CostBudget []string   `json:"cost.budget,omitempty"`
	ModelUse   []string   `json:"model.use,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
}

// Credential is the vendor-neutral credential shape carried in
// job.accepted for provisioned_credentials sessions.
type Credential struct {
	ID          string                 `json:"id"`
	Scheme      string                 `json:"scheme"`
	Value       string                 `json:"value"`
	Endpoint    string                 `json:"endpoint"`
	Profile     string                 `json:"profile,omitempty"`
	Constraints *CredentialConstraints `json:"constraints,omitempty"`
}

// JobEvent is one event in the job's event stream. Body is the
// kind-specific shape; decode it with DecodeEventBody.
type JobEvent struct {
	Kind string          `json:"kind"`
	TS   time.Time       `json:"ts"`
	Body json.RawMessage `json:"body,omitempty"`
}

// ARCPType returns the wire-type string for JobEvent.
func (*JobEvent) ARCPType() string { return TypeJobEvent }

// JobResult is the terminal success payload.
type JobResult struct {
	FinalStatus string          `json:"final_status"`
	Output      json.RawMessage `json:"output,omitempty"`
	ResultID    string          `json:"result_id,omitempty"`
	ResultSize  uint64          `json:"result_size,omitempty"`
	Summary     string          `json:"summary,omitempty"`
}

// ARCPType returns the wire-type string for JobResult.
func (*JobResult) ARCPType() string { return TypeJobResult }

// JobError is the terminal failure payload.
type JobError struct {
	FinalStatus string         `json:"final_status"`
	Code        arcp.ErrorCode `json:"code"`
	Message     string         `json:"message,omitempty"`
	Retryable   bool           `json:"retryable"`
	Details     map[string]any `json:"details,omitempty"`
}

// ARCPType returns the wire-type string for JobError.
func (*JobError) ARCPType() string { return TypeJobError }

// JobCancel is the cancel request from the submitting session.
type JobCancel struct {
	Reason string `json:"reason,omitempty"`
}

// ARCPType returns the wire-type string for JobCancel.
func (*JobCancel) ARCPType() string { return TypeJobCancel }

// JobCancelled is the runtime's acknowledgement of a job.cancel (§7.4),
// emitted before the terminal job.error{code:CANCELLED}.
type JobCancelled struct {
	JobID  string `json:"job_id,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// ARCPType returns the wire-type string for JobCancelled.
func (*JobCancelled) ARCPType() string { return TypeJobCancelled }

// JobSubscribe attaches the current session to an existing job.
type JobSubscribe struct {
	JobID        string `json:"job_id"`
	FromEventSeq uint64 `json:"from_event_seq,omitempty"`
	History      bool   `json:"history,omitempty"`
}

// ARCPType returns the wire-type string for JobSubscribe.
func (*JobSubscribe) ARCPType() string { return TypeJobSubscribe }

// JobSubscribed acknowledges a subscription.
type JobSubscribed struct {
	JobID          string     `json:"job_id"`
	CurrentStatus  string     `json:"current_status"`
	Agent          string     `json:"agent"`
	Lease          arcp.Lease `json:"lease,omitempty"`
	ParentJobID    string     `json:"parent_job_id,omitempty"`
	TraceID        string     `json:"trace_id,omitempty"`
	SubscribedFrom uint64     `json:"subscribed_from"`
	Replayed       bool       `json:"replayed"`
}

// ARCPType returns the wire-type string for JobSubscribed.
func (*JobSubscribed) ARCPType() string { return TypeJobSubscribed }

// JobUnsubscribe detaches a previously attached subscription.
type JobUnsubscribe struct {
	JobID string `json:"job_id"`
}

// ARCPType returns the wire-type string for JobUnsubscribe.
func (*JobUnsubscribe) ARCPType() string { return TypeJobUnsubscribe }

func init() {
	arcp.RegisterMessageType(&JobSubmit{})
	arcp.RegisterMessageType(&JobAccepted{})
	arcp.RegisterMessageType(&JobEvent{})
	arcp.RegisterMessageType(&JobResult{})
	arcp.RegisterMessageType(&JobError{})
	arcp.RegisterMessageType(&JobCancel{})
	arcp.RegisterMessageType(&JobCancelled{})
	arcp.RegisterMessageType(&JobSubscribe{})
	arcp.RegisterMessageType(&JobSubscribed{})
	arcp.RegisterMessageType(&JobUnsubscribe{})
}
