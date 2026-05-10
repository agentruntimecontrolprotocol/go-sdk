package messages

import (
	"encoding/json"
	"time"

	"github.com/fizzpop/arcp-go"
)

// Wire type names for the execution group (RFC §6.2).
const (
	TypeToolInvoke       = "tool.invoke"
	TypeToolResult       = "tool.result"
	TypeToolError        = "tool.error"
	TypeJobAccepted      = "job.accepted"
	TypeJobStarted       = "job.started"
	TypeJobProgress      = "job.progress"
	TypeJobHeartbeat     = "job.heartbeat"
	TypeJobCheckpoint    = "job.checkpoint"
	TypeJobCompleted     = "job.completed"
	TypeJobFailed        = "job.failed"
	TypeJobCancelled     = "job.cancelled"
	TypeJobSchedule      = "job.schedule"
	TypeWorkflowStart    = "workflow.start"
	TypeWorkflowComplete = "workflow.complete"
	TypeAgentDelegate    = "agent.delegate"
	TypeAgentHandoff     = "agent.handoff"
)

// JobState is the lifecycle state of a job (RFC §10.2).
type JobState string

// Defined job states (RFC §10.2). Constants use the "JobState"
// prefix to avoid colliding with the JobAccepted, JobCompleted,
// JobFailed, and JobCancelled message struct types.
const (
	JobStateAccepted  JobState = "accepted"
	JobStateQueued    JobState = "queued"
	JobStateRunning   JobState = "running"
	JobStateBlocked   JobState = "blocked"
	JobStatePaused    JobState = "paused"
	JobStateCompleted JobState = "completed"
	JobStateFailed    JobState = "failed"
	JobStateCancelled JobState = "cancelled"
)

// ToolInvoke commands a tool execution (RFC §6.3, §10).
type ToolInvoke struct {
	Tool      string         `json:"tool"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

// ARCPType returns the wire type name.
func (ToolInvoke) ARCPType() string { return TypeToolInvoke }

// ToolResult is the terminal success event for a direct tool
// invocation (RFC §6.3).
type ToolResult struct {
	Value     json.RawMessage `json:"value,omitempty"`
	ResultRef *ArtifactRef    `json:"result_ref,omitempty"`
}

// ARCPType returns the wire type name.
func (ToolResult) ARCPType() string { return TypeToolResult }

// ToolError is the terminal failure event for a direct tool
// invocation (RFC §6.3, §18.1).
type ToolError struct {
	ErrorPayload
}

// ARCPType returns the wire type name.
func (ToolError) ARCPType() string { return TypeToolError }

// JobAccepted acknowledges a command and reports the assigned job id.
type JobAccepted struct {
	JobID arcp.JobID `json:"job_id"`
}

// ARCPType returns the wire type name.
func (JobAccepted) ARCPType() string { return TypeJobAccepted }

// JobStarted reports that the job has entered the running state.
type JobStarted struct {
	StartedAt time.Time `json:"started_at"`
}

// ARCPType returns the wire type name.
func (JobStarted) ARCPType() string { return TypeJobStarted }

// JobProgress reports incremental progress (RFC §10.1).
type JobProgress struct {
	Percent float64 `json:"percent,omitempty"`
	Message string  `json:"message,omitempty"`
}

// ARCPType returns the wire type name.
func (JobProgress) ARCPType() string { return TypeJobProgress }

// JobHeartbeat reports liveness (RFC §10.3). Sequence increases
// monotonically per job; deadline_ms tells the receiver how long until
// the next heartbeat is overdue.
type JobHeartbeat struct {
	Sequence             int      `json:"sequence"`
	DeadlineMilliseconds int      `json:"deadline_ms"`
	State                JobState `json:"state"`
}

// ARCPType returns the wire type name.
func (JobHeartbeat) ARCPType() string { return TypeJobHeartbeat }

// JobCheckpoint records a recoverable point in job execution
// (RFC §10.1).
type JobCheckpoint struct {
	CheckpointID arcp.CheckpointID `json:"checkpoint_id"`
	Label        string            `json:"label,omitempty"`
	Data         json.RawMessage   `json:"data,omitempty"`
}

// ARCPType returns the wire type name.
func (JobCheckpoint) ARCPType() string { return TypeJobCheckpoint }

// JobCompleted is the terminal success event for a durable job.
type JobCompleted struct {
	Value     json.RawMessage `json:"value,omitempty"`
	ResultRef *ArtifactRef    `json:"result_ref,omitempty"`
}

// ARCPType returns the wire type name.
func (JobCompleted) ARCPType() string { return TypeJobCompleted }

// JobFailed is the terminal failure event for a durable job.
type JobFailed struct {
	ErrorPayload
}

// ARCPType returns the wire type name.
func (JobFailed) ARCPType() string { return TypeJobFailed }

// JobCancelled is the terminal cancellation event for a durable job
// (RFC §10.4).
type JobCancelled struct {
	Reason string `json:"reason,omitempty"`
}

// ARCPType returns the wire type name.
func (JobCancelled) ARCPType() string { return TypeJobCancelled }

// JobScheduleWhen specifies when a scheduled job should execute
// (RFC §10.6). Exactly one of At, Every, After is required.
type JobScheduleWhen struct {
	At           time.Time `json:"at,omitempty"`
	Every        string    `json:"every,omitempty"` // RFC 5545 RRULE
	AfterSeconds int       `json:"after,omitempty"`
}

// JobSchedule requests deferred or recurring execution (RFC §10.6).
// Deferred to v0.2; runtimes return UNIMPLEMENTED.
type JobSchedule struct {
	Job  json.RawMessage `json:"job"`
	When JobScheduleWhen `json:"when"`
}

// ARCPType returns the wire type name.
func (JobSchedule) ARCPType() string { return TypeJobSchedule }

// WorkflowStart begins a workflow (RFC §6.2). Deferred to v0.2.
type WorkflowStart struct {
	Workflow string         `json:"workflow"`
	Inputs   map[string]any `json:"inputs,omitempty"`
}

// ARCPType returns the wire type name.
func (WorkflowStart) ARCPType() string { return TypeWorkflowStart }

// WorkflowComplete reports workflow completion (RFC §6.2).
// Deferred to v0.2.
type WorkflowComplete struct {
	Outputs map[string]any `json:"outputs,omitempty"`
}

// ARCPType returns the wire type name.
func (WorkflowComplete) ARCPType() string { return TypeWorkflowComplete }

// AgentDelegate transfers a sub-task to another agent (RFC §14).
// Deferred to v0.2.
type AgentDelegate struct {
	Target  string                 `json:"target"`
	Task    string                 `json:"task,omitempty"`
	Context map[string]interface{} `json:"context,omitempty"`
}

// ARCPType returns the wire type name.
func (AgentDelegate) ARCPType() string { return TypeAgentDelegate }

// AgentHandoff transfers ownership of a session/job (RFC §14).
// Deferred to v0.2.
type AgentHandoff struct {
	TargetRuntime RuntimeIdentity `json:"target_runtime"`
	SessionID     arcp.SessionID  `json:"session_id,omitempty"`
	JobID         arcp.JobID      `json:"job_id,omitempty"`
}

// ARCPType returns the wire type name.
func (AgentHandoff) ARCPType() string { return TypeAgentHandoff }

func init() {
	register(TypeToolInvoke, func() arcp.MessageType { return &ToolInvoke{} })
	register(TypeToolResult, func() arcp.MessageType { return &ToolResult{} })
	register(TypeToolError, func() arcp.MessageType { return &ToolError{} })
	register(TypeJobAccepted, func() arcp.MessageType { return &JobAccepted{} })
	register(TypeJobStarted, func() arcp.MessageType { return &JobStarted{} })
	register(TypeJobProgress, func() arcp.MessageType { return &JobProgress{} })
	register(TypeJobHeartbeat, func() arcp.MessageType { return &JobHeartbeat{} })
	register(TypeJobCheckpoint, func() arcp.MessageType { return &JobCheckpoint{} })
	register(TypeJobCompleted, func() arcp.MessageType { return &JobCompleted{} })
	register(TypeJobFailed, func() arcp.MessageType { return &JobFailed{} })
	register(TypeJobCancelled, func() arcp.MessageType { return &JobCancelled{} })
	register(TypeJobSchedule, func() arcp.MessageType { return &JobSchedule{} })
	register(TypeWorkflowStart, func() arcp.MessageType { return &WorkflowStart{} })
	register(TypeWorkflowComplete, func() arcp.MessageType { return &WorkflowComplete{} })
	register(TypeAgentDelegate, func() arcp.MessageType { return &AgentDelegate{} })
	register(TypeAgentHandoff, func() arcp.MessageType { return &AgentHandoff{} })
}
