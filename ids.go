package arcp

import (
	"crypto/rand"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"
)

// NewEnvelopeID returns a UUIDv7. UUIDv7 carries a millisecond
// timestamp prefix and is suitable for the envelope id field.
func NewEnvelopeID() string {
	id, err := uuid.NewV7()
	if err != nil {
		// uuid.NewV7 only errors when the entropy source fails;
		// fall back to a v4 rather than panic on a transient.
		id = uuid.New()
	}
	return id.String()
}

// NewULID returns a Crockford-base32 ULID with the current wall-clock
// timestamp. ULIDs sort lexicographically; we use them for
// session/job/result/nonce identifiers so log scans stay ordered.
func NewULID() string {
	return ulid.MustNew(ulid.Timestamp(time.Now()), ulid.DefaultEntropy()).String()
}

// NewSessionID returns a ULID prefixed with "sess_".
func NewSessionID() string {
	return "sess_" + NewULID()
}

// NewJobID returns a ULID prefixed with "job_".
func NewJobID() string {
	return "job_" + NewULID()
}

// NewResultID returns a ULID prefixed with "res_".
func NewResultID() string {
	return "res_" + NewULID()
}

// NewPingNonce returns a ULID prefixed with "p_". The nonce is matched
// on session.pong; ULIDs are short and time-ordered.
func NewPingNonce() string {
	return "p_" + NewULID()
}

// NewCallID returns a ULID prefixed with "c_" for tool_call.call_id.
func NewCallID() string {
	return "c_" + NewULID()
}

// NewTraceID returns a 32-character lowercase hex string suitable for
// the envelope trace_id field (matches W3C trace-context).
func NewTraceID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// rand.Read failure is fatal but we degrade rather than panic.
		return fmt.Sprintf("%032x", time.Now().UnixNano())
	}
	return fmt.Sprintf("%032x", b)
}
