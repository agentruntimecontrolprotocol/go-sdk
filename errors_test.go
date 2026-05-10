package arcp_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/fizzpop/arcp-go"
)

func TestErrorString(t *testing.T) {
	t.Parallel()
	e := arcp.NewError(arcp.CodeInvalidArgument, "bad input")
	got := e.Error()
	if !strings.Contains(got, "INVALID_ARGUMENT") || !strings.Contains(got, "bad input") {
		t.Fatalf("unexpected Error string: %q", got)
	}
	wrapped := e.WithCause(errors.New("root"))
	got = wrapped.Error()
	if !strings.Contains(got, "root") {
		t.Fatalf("wrapped error missing cause: %q", got)
	}
}

func TestNilErrorString(t *testing.T) {
	t.Parallel()
	var e *arcp.Error
	if got := e.Error(); !strings.Contains(got, "nil") {
		t.Fatalf("nil Error.Error() = %q", got)
	}
}

func TestErrorIsBySentinel(t *testing.T) {
	t.Parallel()
	err := arcp.ErrLeaseExpired.WithMessage("lease abc expired").WithDetails(map[string]any{"lease": "abc"})
	if !errors.Is(err, arcp.ErrLeaseExpired) {
		t.Fatalf("errors.Is should match on sentinel by code")
	}
	if errors.Is(err, arcp.ErrLeaseRevoked) {
		t.Fatalf("errors.Is should not match different code")
	}
}

func TestErrorAsExtractsConcrete(t *testing.T) {
	t.Parallel()
	err := arcp.NewError(arcp.CodeResourceExhausted, "rate limited").
		WithDetails(map[string]any{"retry_after_seconds": 30})
	wrapped := fmt.Errorf("upstream: %w", err)
	var concrete *arcp.Error
	if !errors.As(wrapped, &concrete) {
		t.Fatalf("errors.As should extract *arcp.Error")
	}
	if concrete.Code != arcp.CodeResourceExhausted {
		t.Fatalf("expected code RESOURCE_EXHAUSTED, got %q", concrete.Code)
	}
	if v := concrete.Details["retry_after_seconds"]; v != 30 {
		t.Fatalf("expected retry_after_seconds=30, got %v", v)
	}
}

func TestErrorUnwrap(t *testing.T) {
	t.Parallel()
	root := errors.New("root cause")
	e := arcp.NewError(arcp.CodeInternal, "wrap").WithCause(root)
	if !errors.Is(e, root) {
		t.Fatalf("errors.Is should find root via Unwrap")
	}
}

func TestDefaultRetryable(t *testing.T) {
	t.Parallel()
	cases := map[arcp.ErrorCode]bool{
		arcp.CodeResourceExhausted:  true,
		arcp.CodeUnavailable:        true,
		arcp.CodeDeadlineExceeded:   true,
		arcp.CodeInternal:           true,
		arcp.CodeAborted:            true,
		arcp.CodeInvalidArgument:    false,
		arcp.CodePermissionDenied:   false,
		arcp.CodeNotFound:           false,
		arcp.CodeAlreadyExists:      false,
		arcp.CodeFailedPrecondition: false,
		arcp.CodeUnimplemented:      false,
		arcp.CodeUnauthenticated:    false,
		arcp.CodeDataLoss:           false,
	}
	for code, want := range cases {
		if got := arcp.DefaultRetryable(code); got != want {
			t.Errorf("DefaultRetryable(%q) = %v, want %v", code, got, want)
		}
	}
}

func TestIsRetryable(t *testing.T) {
	t.Parallel()
	if !arcp.IsRetryable(arcp.NewError(arcp.CodeUnavailable, "x")) {
		t.Errorf("UNAVAILABLE should be retryable")
	}
	if arcp.IsRetryable(arcp.NewError(arcp.CodeNotFound, "x")) {
		t.Errorf("NOT_FOUND should not be retryable")
	}
	if arcp.IsRetryable(errors.New("plain")) {
		t.Errorf("plain error should not be retryable")
	}
	if arcp.IsRetryable(nil) {
		t.Errorf("nil should not be retryable")
	}
}

func TestCode(t *testing.T) {
	t.Parallel()
	if got := arcp.Code(nil); got != arcp.CodeOK {
		t.Errorf("Code(nil) = %q, want OK", got)
	}
	if got := arcp.Code(errors.New("plain")); got != arcp.CodeUnknown {
		t.Errorf("Code(plain) = %q, want UNKNOWN", got)
	}
	if got := arcp.Code(arcp.ErrLeaseExpired); got != arcp.CodeLeaseExpired {
		t.Errorf("Code(ErrLeaseExpired) = %q, want LEASE_EXPIRED", got)
	}
	wrapped := fmt.Errorf("wrap: %w", arcp.ErrUnauthenticated)
	if got := arcp.Code(wrapped); got != arcp.CodeUnauthenticated {
		t.Errorf("Code(wrapped) = %q, want UNAUTHENTICATED", got)
	}
}

func TestWithDetailsDoesNotMutateOriginal(t *testing.T) {
	t.Parallel()
	base := arcp.NewError(arcp.CodeInternal, "x")
	withA := base.WithDetails(map[string]any{"a": 1})
	withB := base.WithDetails(map[string]any{"b": 2})
	if withA.Details["a"] != 1 || withA.Details["b"] != nil {
		t.Errorf("withA leaked: %+v", withA.Details)
	}
	if withB.Details["b"] != 2 || withB.Details["a"] != nil {
		t.Errorf("withB leaked: %+v", withB.Details)
	}
	if base.Details != nil {
		t.Errorf("base mutated: %+v", base.Details)
	}
}

func TestNilSafeBuilders(t *testing.T) {
	t.Parallel()
	var e *arcp.Error
	if got := e.WithCause(errors.New("x")); got != nil {
		t.Errorf("nil.WithCause should return nil, got %v", got)
	}
	if got := e.WithMessage("x"); got != nil {
		t.Errorf("nil.WithMessage should return nil, got %v", got)
	}
	if got := e.WithDetails(map[string]any{"x": 1}); got != nil {
		t.Errorf("nil.WithDetails should return nil, got %v", got)
	}
	if got := e.Unwrap(); got != nil {
		t.Errorf("nil.Unwrap should return nil, got %v", got)
	}
}
