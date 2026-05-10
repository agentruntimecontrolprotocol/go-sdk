package arcp

import (
	"errors"
	"fmt"
)

// ErrorCode is the canonical error code taxonomy defined in RFC §18.2.
// Implementations MUST use these codes when applicable; deployment-
// specific codes MUST be namespaced (e.g. "arcpx.acme.QUOTA_EXCEEDED").
type ErrorCode string

// Canonical error codes (RFC §18.2). The string values are the wire
// representation.
const (
	CodeOK                   ErrorCode = "OK"
	CodeCancelled            ErrorCode = "CANCELLED"
	CodeUnknown              ErrorCode = "UNKNOWN"
	CodeInvalidArgument      ErrorCode = "INVALID_ARGUMENT"
	CodeDeadlineExceeded     ErrorCode = "DEADLINE_EXCEEDED"
	CodeNotFound             ErrorCode = "NOT_FOUND"
	CodeAlreadyExists        ErrorCode = "ALREADY_EXISTS"
	CodePermissionDenied     ErrorCode = "PERMISSION_DENIED"
	CodeResourceExhausted    ErrorCode = "RESOURCE_EXHAUSTED"
	CodeFailedPrecondition   ErrorCode = "FAILED_PRECONDITION"
	CodeAborted              ErrorCode = "ABORTED"
	CodeOutOfRange           ErrorCode = "OUT_OF_RANGE"
	CodeUnimplemented        ErrorCode = "UNIMPLEMENTED"
	CodeInternal             ErrorCode = "INTERNAL"
	CodeUnavailable          ErrorCode = "UNAVAILABLE"
	CodeDataLoss             ErrorCode = "DATA_LOSS"
	CodeUnauthenticated      ErrorCode = "UNAUTHENTICATED"
	CodeHeartbeatLost        ErrorCode = "HEARTBEAT_LOST"
	CodeLeaseExpired         ErrorCode = "LEASE_EXPIRED"
	CodeLeaseRevoked         ErrorCode = "LEASE_REVOKED"
	CodeBackpressureOverflow ErrorCode = "BACKPRESSURE_OVERFLOW"
)

// Error is the structured error type used throughout this
// implementation (RFC §18.1). Wraps a cause via Unwrap so
// errors.Is/As/Unwrap work as expected.
type Error struct {
	// Code is the canonical or namespaced error code.
	Code ErrorCode
	// Message is a human-readable description; optional but recommended.
	Message string
	// Retryable indicates whether the operation MAY succeed on retry.
	// Defaults are filled in by NewError per RFC §18.3.
	Retryable bool
	// Details carries free-form key/value details, e.g.
	// {"retry_after_seconds": 30}.
	Details map[string]any
	// Cause is the wrapped error (chained per RFC §18.1).
	Cause error
}

// Error implements the standard library's error interface.
func (e *Error) Error() string {
	if e == nil {
		return "<nil arcp.Error>"
	}
	if e.Cause != nil {
		return fmt.Sprintf("arcp: [%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("arcp: [%s] %s", e.Code, e.Message)
}

// Unwrap returns the wrapped cause for compatibility with errors.Unwrap.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// Is implements the matching protocol used by errors.Is. Two
// arcp.Errors match when their Codes are equal. Sentinels declared in
// this package compare on Code only; concrete details are inspected via
// errors.As.
func (e *Error) Is(target error) bool {
	if e == nil {
		return target == nil
	}
	var t *Error
	if !errors.As(target, &t) {
		return false
	}
	return e.Code == t.Code
}

// WithCause returns a copy of e with Cause set.
func (e *Error) WithCause(cause error) *Error {
	if e == nil {
		return nil
	}
	c := *e
	c.Cause = cause
	return &c
}

// WithMessage returns a copy of e with the given message.
func (e *Error) WithMessage(msg string) *Error {
	if e == nil {
		return nil
	}
	c := *e
	c.Message = msg
	return &c
}

// WithDetails returns a copy of e with the given details merged in.
// The original details map (if any) is not mutated.
func (e *Error) WithDetails(details map[string]any) *Error {
	if e == nil {
		return nil
	}
	c := *e
	merged := make(map[string]any, len(e.Details)+len(details))
	for k, v := range e.Details {
		merged[k] = v
	}
	for k, v := range details {
		merged[k] = v
	}
	c.Details = merged
	return &c
}

// NewError constructs an Error with the given code and message.
// Retryable is initialized from DefaultRetryable(code).
func NewError(code ErrorCode, msg string) *Error {
	return &Error{Code: code, Message: msg, Retryable: DefaultRetryable(code)}
}

// DefaultRetryable returns the default retryability for a code per
// RFC §18.3.
func DefaultRetryable(code ErrorCode) bool {
	switch code {
	case CodeResourceExhausted, CodeUnavailable, CodeDeadlineExceeded, CodeInternal, CodeAborted:
		return true
	default:
		return false
	}
}

// IsRetryable returns true if err (or any wrapped arcp.Error in its
// chain) reports Retryable. Non-arcp errors return false.
func IsRetryable(err error) bool {
	var e *Error
	if errors.As(err, &e) {
		return e.Retryable
	}
	return false
}

// Code returns the ErrorCode of err if err is or wraps an arcp.Error,
// or CodeUnknown otherwise.
func Code(err error) ErrorCode {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	if err == nil {
		return CodeOK
	}
	return CodeUnknown
}

// Sentinel errors. Use errors.Is to compare. Each sentinel is a
// distinct *Error keyed on its canonical Code.
var (
	// ErrUnauthenticated indicates missing or invalid credentials
	// (RFC §18.2).
	ErrUnauthenticated = &Error{Code: CodeUnauthenticated, Message: "unauthenticated"}
	// ErrPermissionDenied indicates the caller lacks the required
	// permission or lease (RFC §15, §18.2).
	ErrPermissionDenied = &Error{Code: CodePermissionDenied, Message: "permission denied"}
	// ErrLeaseExpired indicates an operation attempted with an expired
	// lease (RFC §15.5).
	ErrLeaseExpired = &Error{Code: CodeLeaseExpired, Message: "lease expired"}
	// ErrLeaseRevoked indicates an operation attempted with a revoked
	// lease (RFC §15.5).
	ErrLeaseRevoked = &Error{Code: CodeLeaseRevoked, Message: "lease revoked"}
	// ErrUnimplemented indicates the runtime does not support the
	// requested feature (RFC §18.2). v0.1 returns this for deferred
	// surfaces (mtls, oauth2, sidecar binary, scheduled jobs, etc.).
	ErrUnimplemented = &Error{Code: CodeUnimplemented, Message: "not implemented in this runtime"}
	// ErrDeadlineExceeded indicates an operation timed out
	// (RFC §18.2). Retryable by default.
	ErrDeadlineExceeded = &Error{Code: CodeDeadlineExceeded, Message: "deadline exceeded", Retryable: true}
	// ErrCancelled indicates the operation was cancelled
	// (RFC §10.4, §18.2).
	ErrCancelled = &Error{Code: CodeCancelled, Message: "cancelled"}
	// ErrNotFound indicates a referenced entity does not exist
	// (RFC §18.2).
	ErrNotFound = &Error{Code: CodeNotFound, Message: "not found"}
	// ErrAlreadyExists indicates an entity creation conflicted
	// (RFC §18.2). Used for duplicate envelope ids in the event log.
	ErrAlreadyExists = &Error{Code: CodeAlreadyExists, Message: "already exists"}
	// ErrInvalidArgument indicates a malformed argument (RFC §18.2).
	ErrInvalidArgument = &Error{Code: CodeInvalidArgument, Message: "invalid argument"}
	// ErrInternal indicates an internal runtime error (RFC §18.2).
	ErrInternal = &Error{Code: CodeInternal, Message: "internal error", Retryable: true}
	// ErrBackpressureOverflow indicates a stream or subscription was
	// dropped due to overflow (RFC §18.2).
	ErrBackpressureOverflow = &Error{Code: CodeBackpressureOverflow, Message: "backpressure overflow"}
	// ErrHeartbeatLost indicates a job missed required heartbeats
	// (RFC §10.3, §18.2).
	ErrHeartbeatLost = &Error{Code: CodeHeartbeatLost, Message: "heartbeat lost"}
)
