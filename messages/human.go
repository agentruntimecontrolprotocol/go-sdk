package messages

import (
	"encoding/json"
	"time"

	"github.com/fizzpop/arcp-go"
)

// Wire type names for the human-in-the-loop group (RFC §6.2, §12).
const (
	TypeHumanInputRequest   = "human.input.request"
	TypeHumanInputResponse  = "human.input.response"
	TypeHumanChoiceRequest  = "human.choice.request"
	TypeHumanChoiceResponse = "human.choice.response"
	TypeHumanInputCancelled = "human.input.cancelled"
)

// HumanInputRequest asks a human for free-form structured input
// (RFC §12.1). The job is moved to `blocked` until a response arrives
// or ExpiresAt elapses.
type HumanInputRequest struct {
	Prompt         string          `json:"prompt"`
	ResponseSchema json.RawMessage `json:"response_schema,omitempty"`
	Default        json.RawMessage `json:"default,omitempty"`
	ExpiresAt      time.Time       `json:"expires_at"`
}

// ARCPType returns the wire type name.
func (HumanInputRequest) ARCPType() string { return TypeHumanInputRequest }

// HumanInputResponse carries the human's reply (RFC §12.1).
// CorrelationID on the envelope ties the response to the request.
type HumanInputResponse struct {
	Value       json.RawMessage `json:"value"`
	RespondedBy string          `json:"responded_by,omitempty"`
	RespondedAt time.Time       `json:"responded_at,omitempty"`
}

// ARCPType returns the wire type name.
func (HumanInputResponse) ARCPType() string { return TypeHumanInputResponse }

// HumanChoiceOption is one selectable option in a choice request
// (RFC §12.2).
type HumanChoiceOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// HumanChoiceRequest asks the human to pick one of a set (RFC §12.2).
type HumanChoiceRequest struct {
	Prompt    string              `json:"prompt"`
	Options   []HumanChoiceOption `json:"options"`
	ExpiresAt time.Time           `json:"expires_at"`
}

// ARCPType returns the wire type name.
func (HumanChoiceRequest) ARCPType() string { return TypeHumanChoiceRequest }

// HumanChoiceResponse carries the human's choice (RFC §12.2).
type HumanChoiceResponse struct {
	ChoiceID    string    `json:"choice_id"`
	RespondedBy string    `json:"responded_by,omitempty"`
	RespondedAt time.Time `json:"responded_at,omitempty"`
}

// ARCPType returns the wire type name.
func (HumanChoiceResponse) ARCPType() string { return TypeHumanChoiceResponse }

// HumanInputCancelled is sent when a request expires without a
// response (RFC §12.4) or is otherwise withdrawn.
type HumanInputCancelled struct {
	Code   arcp.ErrorCode `json:"code"`
	Reason string         `json:"reason,omitempty"`
}

// ARCPType returns the wire type name.
func (HumanInputCancelled) ARCPType() string { return TypeHumanInputCancelled }

func init() {
	register(TypeHumanInputRequest, func() arcp.MessageType { return &HumanInputRequest{} })
	register(TypeHumanInputResponse, func() arcp.MessageType { return &HumanInputResponse{} })
	register(TypeHumanChoiceRequest, func() arcp.MessageType { return &HumanChoiceRequest{} })
	register(TypeHumanChoiceResponse, func() arcp.MessageType { return &HumanChoiceResponse{} })
	register(TypeHumanInputCancelled, func() arcp.MessageType { return &HumanInputCancelled{} })
}
