package arcp

import (
	"errors"
	"fmt"
)

// ErrorCode is the canonical string code for the protocol's error
// taxonomy. The fifteen codes below cover every spec-mandated rejection
// point.
type ErrorCode string

// The full set of canonical error codes.
const (
	CodePermissionDenied         ErrorCode = "PERMISSION_DENIED"
	CodeLeaseSubsetViolation     ErrorCode = "LEASE_SUBSET_VIOLATION"
	CodeJobNotFound              ErrorCode = "JOB_NOT_FOUND"
	CodeDuplicateKey             ErrorCode = "DUPLICATE_KEY"
	CodeAgentNotAvailable        ErrorCode = "AGENT_NOT_AVAILABLE"
	CodeAgentVersionNotAvailable ErrorCode = "AGENT_VERSION_NOT_AVAILABLE"
	CodeCancelled                ErrorCode = "CANCELLED"
	CodeTimeout                  ErrorCode = "TIMEOUT"
	CodeResumeWindowExpired      ErrorCode = "RESUME_WINDOW_EXPIRED"
	CodeHeartbeatLost            ErrorCode = "HEARTBEAT_LOST"
	CodeLeaseExpired             ErrorCode = "LEASE_EXPIRED"
	CodeBudgetExhausted          ErrorCode = "BUDGET_EXHAUSTED"
	CodeInvalidRequest           ErrorCode = "INVALID_REQUEST"
	CodeUnauthenticated          ErrorCode = "UNAUTHENTICATED"
	CodeInternalError            ErrorCode = "INTERNAL_ERROR"
)

// Error is the structured ARCP error. It carries a code, a human
// message, a retry hint, optional structured details, and an
// optional wrapped cause.
type Error struct {
	Code      ErrorCode      `json:"code"`
	Message   string         `json:"message"`
	Retryable bool           `json:"retryable"`
	Details   map[string]any `json:"details,omitempty"`
	cause     error
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e.Message == "" {
		return string(e.Code)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the wrapped cause, if any.
func (e *Error) Unwrap() error { return e.cause }

// Is matches by Code so wrapped errors satisfy errors.Is against
// sentinel values.
func (e *Error) Is(target error) bool {
	var t *Error
	if !errors.As(target, &t) {
		return false
	}
	return e.Code == t.Code
}

// WithCause returns a copy of e wrapping cause.
func (e *Error) WithCause(cause error) *Error {
	c := *e
	c.cause = cause
	return &c
}

// WithMessage returns a copy of e with msg as the human message.
func (e *Error) WithMessage(msg string) *Error {
	c := *e
	c.Message = msg
	return &c
}

// WithDetails returns a copy of e with details merged into Details.
func (e *Error) WithDetails(details map[string]any) *Error {
	c := *e
	if c.Details == nil {
		c.Details = map[string]any{}
	}
	for k, v := range details {
		c.Details[k] = v
	}
	return &c
}

// Sentinel error values. Each maps to one ErrorCode. Use errors.Is
// against these to test for a particular code.
var (
	ErrPermissionDenied         = &Error{Code: CodePermissionDenied, Message: "permission denied", Retryable: false}
	ErrLeaseSubsetViolation     = &Error{Code: CodeLeaseSubsetViolation, Message: "lease subset violation", Retryable: false}
	ErrJobNotFound              = &Error{Code: CodeJobNotFound, Message: "job not found", Retryable: false}
	ErrDuplicateKey             = &Error{Code: CodeDuplicateKey, Message: "duplicate idempotency key", Retryable: false}
	ErrAgentNotAvailable        = &Error{Code: CodeAgentNotAvailable, Message: "agent not available", Retryable: false}
	ErrAgentVersionNotAvailable = &Error{Code: CodeAgentVersionNotAvailable, Message: "agent version not available", Retryable: false}
	ErrCancelled                = &Error{Code: CodeCancelled, Message: "cancelled", Retryable: false}
	ErrTimeout                  = &Error{Code: CodeTimeout, Message: "timeout", Retryable: false}
	ErrResumeWindowExpired      = &Error{Code: CodeResumeWindowExpired, Message: "resume window expired", Retryable: false}
	ErrHeartbeatLost            = &Error{Code: CodeHeartbeatLost, Message: "heartbeat lost", Retryable: true}
	ErrLeaseExpired             = &Error{Code: CodeLeaseExpired, Message: "lease expired", Retryable: false}
	ErrBudgetExhausted          = &Error{Code: CodeBudgetExhausted, Message: "budget exhausted", Retryable: false}
	ErrInvalidRequest           = &Error{Code: CodeInvalidRequest, Message: "invalid request", Retryable: false}
	ErrUnauthenticated          = &Error{Code: CodeUnauthenticated, Message: "unauthenticated", Retryable: false}
	ErrInternalError            = &Error{Code: CodeInternalError, Message: "internal error", Retryable: true}
)

// Code walks the error chain and returns the first matched ErrorCode.
// If no *Error is found, Code returns CodeInternalError.
func Code(err error) ErrorCode {
	if err == nil {
		return ""
	}
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return CodeInternalError
}

// IsRetryable reports whether err is structurally marked retryable.
// Non-arcp errors are conservatively reported as retryable so generic
// transport-level failures do not become fatal.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	var e *Error
	if errors.As(err, &e) {
		return e.Retryable
	}
	return true
}

// Newf constructs an *Error with code and a formatted message.
func Newf(code ErrorCode, format string, args ...any) *Error {
	return &Error{
		Code:      code,
		Message:   fmt.Sprintf(format, args...),
		Retryable: defaultRetryable(code),
	}
}

func defaultRetryable(code ErrorCode) bool {
	switch code {
	case CodeInternalError, CodeHeartbeatLost:
		return true
	default:
		return false
	}
}
