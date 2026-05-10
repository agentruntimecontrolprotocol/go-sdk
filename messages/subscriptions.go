package messages

import (
	"encoding/json"

	"github.com/agentruntimecontrolprotocol/go-sdk"
)

// Wire type names for the subscription group (RFC §6.2, §13).
const (
	TypeSubscribe         = "subscribe"
	TypeSubscribeAccepted = "subscribe.accepted"
	TypeSubscribeEvent    = "subscribe.event"
	TypeUnsubscribe       = "unsubscribe"
	TypeSubscribeClosed   = "subscribe.closed"
)

// SubscribeFilter narrows the events delivered on a subscription
// (RFC §13.2). Conditions across fields are AND-ed; arrays within a
// field are OR-ed.
type SubscribeFilter struct {
	SessionID   []arcp.SessionID `json:"session_id,omitempty"`
	TraceID     []arcp.TraceID   `json:"trace_id,omitempty"`
	JobID       []arcp.JobID     `json:"job_id,omitempty"`
	StreamID    []arcp.StreamID  `json:"stream_id,omitempty"`
	Types       []string         `json:"types,omitempty"`
	MinPriority arcp.Priority    `json:"min_priority,omitempty"`
}

// SubscribeSince specifies a backfill point (RFC §13.3).
type SubscribeSince struct {
	AfterMessageID arcp.MessageID `json:"after_message_id,omitempty"`
}

// Subscribe opens an event subscription (RFC §13.1).
type Subscribe struct {
	Filter SubscribeFilter `json:"filter"`
	Since  *SubscribeSince `json:"since,omitempty"`
}

// ARCPType returns the wire type name.
func (Subscribe) ARCPType() string { return TypeSubscribe }

// SubscribeAccepted is the runtime's acknowledgement of a Subscribe
// (RFC §13.1).
type SubscribeAccepted struct {
	SubscriptionID arcp.SubscriptionID `json:"subscription_id"`
}

// ARCPType returns the wire type name.
func (SubscribeAccepted) ARCPType() string { return TypeSubscribeAccepted }

// SubscribeEvent wraps a delivered event (RFC §13.1). Event holds the
// underlying envelope JSON; consumers can decode it with
// arcp.Envelope.UnmarshalJSON.
type SubscribeEvent struct {
	Event json.RawMessage `json:"event"`
}

// ARCPType returns the wire type name.
func (SubscribeEvent) ARCPType() string { return TypeSubscribeEvent }

// Unsubscribe ends a subscription (RFC §13.4).
type Unsubscribe struct {
	SubscriptionID arcp.SubscriptionID `json:"subscription_id"`
}

// ARCPType returns the wire type name.
func (Unsubscribe) ARCPType() string { return TypeUnsubscribe }

// SubscribeClosed indicates the runtime ended the subscription
// unilaterally (RFC §13.4).
type SubscribeClosed struct {
	SubscriptionID arcp.SubscriptionID `json:"subscription_id"`
	Code           arcp.ErrorCode      `json:"code,omitempty"`
	Reason         string              `json:"reason,omitempty"`
}

// ARCPType returns the wire type name.
func (SubscribeClosed) ARCPType() string { return TypeSubscribeClosed }

func init() {
	register(TypeSubscribe, func() arcp.MessageType { return &Subscribe{} })
	register(TypeSubscribeAccepted, func() arcp.MessageType { return &SubscribeAccepted{} })
	register(TypeSubscribeEvent, func() arcp.MessageType { return &SubscribeEvent{} })
	register(TypeUnsubscribe, func() arcp.MessageType { return &Unsubscribe{} })
	register(TypeSubscribeClosed, func() arcp.MessageType { return &SubscribeClosed{} })
}
