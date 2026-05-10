package messages

import (
	"time"

	"github.com/agentruntimecontrolprotocol/go-sdk"
)

// Wire type names for the session-handshake group (RFC §6.2).
const (
	TypeSessionOpen            = "session.open"
	TypeSessionChallenge       = "session.challenge"
	TypeSessionAuthenticate    = "session.authenticate"
	TypeSessionAccepted        = "session.accepted"
	TypeSessionUnauthenticated = "session.unauthenticated"
	TypeSessionRejected        = "session.rejected"
	TypeSessionRefresh         = "session.refresh"
	TypeSessionEvicted         = "session.evicted"
	TypeSessionClose           = "session.close"
)

// SessionOpen is sent by the client to begin the four-step handshake
// (RFC §8.1).
type SessionOpen struct {
	Auth         Auth           `json:"auth"`
	Client       ClientIdentity `json:"client"`
	Capabilities Capabilities   `json:"capabilities,omitempty"`
}

// ARCPType returns the wire type name.
func (SessionOpen) ARCPType() string { return TypeSessionOpen }

// SessionChallenge is sent by the runtime when additional credentials
// are required (RFC §8.1).
type SessionChallenge struct {
	Challenge string    `json:"challenge"`
	Scheme    string    `json:"scheme,omitempty"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
}

// ARCPType returns the wire type name.
func (SessionChallenge) ARCPType() string { return TypeSessionChallenge }

// SessionAuthenticate is the client's response to a challenge
// (RFC §8.1).
type SessionAuthenticate struct {
	Auth      Auth   `json:"auth"`
	Challenge string `json:"challenge,omitempty"`
}

// ARCPType returns the wire type name.
func (SessionAuthenticate) ARCPType() string { return TypeSessionAuthenticate }

// SessionAccepted concludes a successful handshake (RFC §8.1, §8.3).
type SessionAccepted struct {
	SessionID    arcp.SessionID  `json:"session_id"`
	Runtime      RuntimeIdentity `json:"runtime,omitempty"`
	Capabilities Capabilities    `json:"capabilities,omitempty"`
	Lease        *Lease          `json:"lease,omitempty"`
}

// ARCPType returns the wire type name.
func (SessionAccepted) ARCPType() string { return TypeSessionAccepted }

// SessionUnauthenticated indicates that the offered credentials are
// invalid or expired (RFC §8).
type SessionUnauthenticated struct {
	Code    arcp.ErrorCode `json:"code"`
	Message string         `json:"message"`
}

// ARCPType returns the wire type name.
func (SessionUnauthenticated) ARCPType() string { return TypeSessionUnauthenticated }

// SessionRejected ends the handshake when negotiation fails — for
// example, when a required capability is unsupported (RFC §7).
type SessionRejected struct {
	Code    arcp.ErrorCode `json:"code"`
	Message string         `json:"message"`
}

// ARCPType returns the wire type name.
func (SessionRejected) ARCPType() string { return TypeSessionRejected }

// SessionRefresh requests re-authentication mid-session (RFC §8.4).
type SessionRefresh struct {
	Reason    string    `json:"reason,omitempty"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
}

// ARCPType returns the wire type name.
func (SessionRefresh) ARCPType() string { return TypeSessionRefresh }

// SessionEvicted indicates the runtime terminated the session
// (RFC §8.5).
type SessionEvicted struct {
	Code    arcp.ErrorCode `json:"code"`
	Reason  string         `json:"reason,omitempty"`
	Message string         `json:"message,omitempty"`
}

// ARCPType returns the wire type name.
func (SessionEvicted) ARCPType() string { return TypeSessionEvicted }

// SessionClose is the graceful close request (RFC §9).
type SessionClose struct {
	Reason string `json:"reason,omitempty"`
	Detach bool   `json:"detach,omitempty"`
}

// ARCPType returns the wire type name.
func (SessionClose) ARCPType() string { return TypeSessionClose }

func init() {
	register(TypeSessionOpen, func() arcp.MessageType { return &SessionOpen{} })
	register(TypeSessionChallenge, func() arcp.MessageType { return &SessionChallenge{} })
	register(TypeSessionAuthenticate, func() arcp.MessageType { return &SessionAuthenticate{} })
	register(TypeSessionAccepted, func() arcp.MessageType { return &SessionAccepted{} })
	register(TypeSessionUnauthenticated, func() arcp.MessageType { return &SessionUnauthenticated{} })
	register(TypeSessionRejected, func() arcp.MessageType { return &SessionRejected{} })
	register(TypeSessionRefresh, func() arcp.MessageType { return &SessionRefresh{} })
	register(TypeSessionEvicted, func() arcp.MessageType { return &SessionEvicted{} })
	register(TypeSessionClose, func() arcp.MessageType { return &SessionClose{} })
}
