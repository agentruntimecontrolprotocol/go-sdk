package messages

// Wire-type tokens. The constants below are the canonical strings used
// in Envelope.Type. Group, payload struct, and registration table all
// key off these.
const (
	TypeSessionHello    = "session.hello"
	TypeSessionWelcome  = "session.welcome"
	TypeSessionError    = "session.error"
	TypeSessionClose    = "session.close"
	TypeSessionClosed   = "session.closed"
	TypeSessionPing     = "session.ping"
	TypeSessionPong     = "session.pong"
	TypeSessionAck      = "session.ack"
	TypeSessionListJobs = "session.list_jobs"
	TypeSessionJobs     = "session.jobs"

	TypeJobSubmit      = "job.submit"
	TypeJobAccepted    = "job.accepted"
	TypeJobEvent       = "job.event"
	TypeJobResult      = "job.result"
	TypeJobError       = "job.error"
	TypeJobCancel      = "job.cancel"
	TypeJobCancelled   = "job.cancelled"
	TypeJobSubscribe   = "job.subscribe"
	TypeJobSubscribed  = "job.subscribed"
	TypeJobUnsubscribe = "job.unsubscribe"
)

// Event kinds carried inside JobEvent.Kind. The eight reserved kinds
// (per spec §8.2) plus the v1.1 additions (progress, result_chunk).
const (
	KindLog         = "log"
	KindThought     = "thought"
	KindToolCall    = "tool_call"
	KindToolResult  = "tool_result"
	KindStatus      = "status"
	KindMetric      = "metric"
	KindArtifactRef = "artifact_ref"
	KindDelegate    = "delegate"
	KindProgress    = "progress"
	KindResultChunk = "result_chunk"
)

// JobStatus values used in session.jobs and job.subscribed payloads.
const (
	StatusPending   = "pending"
	StatusRunning   = "running"
	StatusSuccess   = "success"
	StatusError     = "error"
	StatusCancelled = "cancelled"
	StatusTimedOut  = "timed_out"
)

// Reserved status event phases.
const (
	PhaseCredentialRotated = "credential_rotated"
)
