package messages

import (
	"github.com/fizzpop/arcp-go"
)

// Wire type names for the control group (RFC §6.2).
const (
	TypePing              = "ping"
	TypePong              = "pong"
	TypeAck               = "ack"
	TypeNack              = "nack"
	TypeCancel            = "cancel"
	TypeCancelAccepted    = "cancel.accepted"
	TypeCancelRefused     = "cancel.refused"
	TypeInterrupt         = "interrupt"
	TypeResume            = "resume"
	TypeBackpressure      = "backpressure"
	TypeCheckpointCreate  = "checkpoint.create"
	TypeCheckpointRestore = "checkpoint.restore"
)

// Ping is a liveness probe (RFC §6.2 control).
type Ping struct {
	Note string `json:"note,omitempty"`
}

// ARCPType returns the wire type name.
func (Ping) ARCPType() string { return TypePing }

// Pong is the response to a Ping.
type Pong struct {
	Note string `json:"note,omitempty"`
}

// ARCPType returns the wire type name.
func (Pong) ARCPType() string { return TypePong }

// Ack acknowledges receipt of a command. The CorrelationID on the
// envelope identifies the command being acknowledged.
type Ack struct{}

// ARCPType returns the wire type name.
func (Ack) ARCPType() string { return TypeAck }

// Nack rejects a command with a structured error (RFC §6.3).
type Nack struct {
	Code    arcp.ErrorCode `json:"code"`
	Message string         `json:"message,omitempty"`
	Details map[string]any `json:"details,omitempty"`
}

// ARCPType returns the wire type name.
func (Nack) ARCPType() string { return TypeNack }

// Cancel requests termination of a job, stream, or session (RFC §10.4).
type Cancel struct {
	Target     CancelTarget `json:"target"`
	TargetID   string       `json:"target_id"`
	Reason     string       `json:"reason,omitempty"`
	DeadlineMS int          `json:"deadline_ms,omitempty"`
}

// ARCPType returns the wire type name.
func (Cancel) ARCPType() string { return TypeCancel }

// CancelAccepted acknowledges a cancel request (RFC §10.4).
type CancelAccepted struct {
	Target   CancelTarget `json:"target"`
	TargetID string       `json:"target_id"`
}

// ARCPType returns the wire type name.
func (CancelAccepted) ARCPType() string { return TypeCancelAccepted }

// CancelRefused rejects a cancel request (RFC §10.4).
type CancelRefused struct {
	Target   CancelTarget   `json:"target"`
	TargetID string         `json:"target_id"`
	Code     arcp.ErrorCode `json:"code"`
	Reason   string         `json:"reason,omitempty"`
}

// ARCPType returns the wire type name.
func (CancelRefused) ARCPType() string { return TypeCancelRefused }

// Interrupt requests a job pause for human guidance (RFC §10.5).
type Interrupt struct {
	Target   CancelTarget `json:"target"`
	TargetID string       `json:"target_id"`
	Prompt   string       `json:"prompt,omitempty"`
}

// ARCPType returns the wire type name.
func (Interrupt) ARCPType() string { return TypeInterrupt }

// Resume reconnects to an existing session and asks for replay from
// the last observed message id (RFC §19).
type Resume struct {
	AfterMessageID     arcp.MessageID    `json:"after_message_id,omitempty"`
	CheckpointID       arcp.CheckpointID `json:"checkpoint_id,omitempty"`
	IncludeOpenStreams bool              `json:"include_open_streams,omitempty"`
}

// ARCPType returns the wire type name.
func (Resume) ARCPType() string { return TypeResume }

// Backpressure asks the peer to slow a stream (RFC §11.2).
type Backpressure struct {
	DesiredRatePerSecond int    `json:"desired_rate_per_second,omitempty"`
	BufferRemainingBytes int    `json:"buffer_remaining_bytes,omitempty"`
	Reason               string `json:"reason,omitempty"`
}

// ARCPType returns the wire type name.
func (Backpressure) ARCPType() string { return TypeBackpressure }

// CheckpointCreate requests a job checkpoint (RFC §10.1).
type CheckpointCreate struct {
	Label string `json:"label,omitempty"`
}

// ARCPType returns the wire type name.
func (CheckpointCreate) ARCPType() string { return TypeCheckpointCreate }

// CheckpointRestore restores a job from a stored checkpoint
// (RFC §10.1, §19). Deferred to v0.2; the message type is registered
// so unknown-type policy applies correctly.
type CheckpointRestore struct {
	CheckpointID arcp.CheckpointID `json:"checkpoint_id"`
}

// ARCPType returns the wire type name.
func (CheckpointRestore) ARCPType() string { return TypeCheckpointRestore }

func init() {
	register(TypePing, func() arcp.MessageType { return &Ping{} })
	register(TypePong, func() arcp.MessageType { return &Pong{} })
	register(TypeAck, func() arcp.MessageType { return &Ack{} })
	register(TypeNack, func() arcp.MessageType { return &Nack{} })
	register(TypeCancel, func() arcp.MessageType { return &Cancel{} })
	register(TypeCancelAccepted, func() arcp.MessageType { return &CancelAccepted{} })
	register(TypeCancelRefused, func() arcp.MessageType { return &CancelRefused{} })
	register(TypeInterrupt, func() arcp.MessageType { return &Interrupt{} })
	register(TypeResume, func() arcp.MessageType { return &Resume{} })
	register(TypeBackpressure, func() arcp.MessageType { return &Backpressure{} })
	register(TypeCheckpointCreate, func() arcp.MessageType { return &CheckpointCreate{} })
	register(TypeCheckpointRestore, func() arcp.MessageType { return &CheckpointRestore{} })
}
