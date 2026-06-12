package arcp_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
	_ "github.com/agentruntimecontrolprotocol/go-sdk/messages"
)

// Coverage-focused unit tests for the small primitives in the root
// package.

func TestNewEnvelopeAllocatesIDsAndType(t *testing.T) {
	env, err := arcp.NewEnvelope("session.hello", map[string]any{"k": "v"})
	if err != nil {
		t.Fatal(err)
	}
	if env.ARCP != arcp.ProtocolVersion {
		t.Fatalf("ARCP = %q, want %q", env.ARCP, arcp.ProtocolVersion)
	}
	if env.ID == "" {
		t.Fatal("ID must be allocated")
	}
	if env.Type != "session.hello" {
		t.Fatalf("Type = %s", env.Type)
	}
	if len(env.Payload) == 0 {
		t.Fatal("payload should be encoded")
	}
}

func TestMarshalPayloadPassthroughs(t *testing.T) {
	// nil
	b, err := arcp.MarshalPayload(nil)
	if err != nil || b != nil {
		t.Fatalf("nil: b=%v err=%v", b, err)
	}
	// RawMessage
	raw := json.RawMessage(`{"x":1}`)
	b, err = arcp.MarshalPayload(raw)
	if err != nil || string(b) != `{"x":1}` {
		t.Fatalf("RawMessage: %s %v", b, err)
	}
	// []byte passes through unchanged
	b, err = arcp.MarshalPayload([]byte(`{"y":2}`))
	if err != nil || string(b) != `{"y":2}` {
		t.Fatalf("[]byte: %s %v", b, err)
	}
}

func TestDecodePayloadEmpty(t *testing.T) {
	env := arcp.Envelope{}
	var v map[string]any
	if err := env.DecodePayload(&v); err != nil {
		t.Fatalf("empty decode should be a no-op, got %v", err)
	}
}

func TestErrorChaining(t *testing.T) {
	cause := errors.New("disk full")
	wrapped := arcp.ErrInternalError.WithCause(cause).WithMessage("retry later")
	if !errors.Is(wrapped, arcp.ErrInternalError) {
		t.Fatal("errors.Is must match by code")
	}
	if !strings.Contains(wrapped.Error(), "INTERNAL_ERROR") {
		t.Fatalf("Error() = %s", wrapped.Error())
	}
	if errors.Unwrap(wrapped) == nil {
		t.Fatal("Unwrap must return wrapped cause")
	}
}

func TestErrorWithDetailsMerges(t *testing.T) {
	e := arcp.ErrInvalidRequest.WithDetails(map[string]any{"a": 1}).WithDetails(map[string]any{"b": 2})
	if e.Details["a"] != 1 || e.Details["b"] != 2 {
		t.Fatalf("details merge broken: %v", e.Details)
	}
}

func TestNewfDefaultRetryable(t *testing.T) {
	e := arcp.Newf(arcp.CodeInternalError, "boom %d", 1)
	if !e.Retryable {
		t.Fatal("INTERNAL_ERROR should be retryable by default")
	}
	if e.Error() != "INTERNAL_ERROR: boom 1" {
		t.Fatalf("Error() = %s", e.Error())
	}
	if !arcp.Newf(arcp.CodeHeartbeatLost, "lost").Retryable {
		t.Fatal("HEARTBEAT_LOST should be retryable by default")
	}
	if arcp.Newf(arcp.CodePermissionDenied, "no").Retryable {
		t.Fatal("PERMISSION_DENIED should not be retryable")
	}
}

func TestHasFeature(t *testing.T) {
	if !arcp.HasFeature([]string{"x", "y"}, "y") {
		t.Fatal()
	}
	if arcp.HasFeature([]string{"x"}, "z") {
		t.Fatal()
	}
}

func TestIDGenerators(t *testing.T) {
	a := arcp.NewSessionID()
	b := arcp.NewJobID()
	c := arcp.NewResultID()
	d := arcp.NewPingNonce()
	e := arcp.NewCallID()
	for name, id := range map[string]string{"sess": a, "job": b, "res": c, "ping": d, "call": e} {
		if id == "" {
			t.Fatalf("%s ID empty", name)
		}
	}
	if !strings.HasPrefix(a, "sess_") {
		t.Fatalf("session id missing prefix: %s", a)
	}
	if !strings.HasPrefix(b, "job_") {
		t.Fatalf("job id missing prefix: %s", b)
	}
	if !strings.HasPrefix(c, "res_") {
		t.Fatalf("result id missing prefix: %s", c)
	}
	trace := arcp.NewTraceID()
	if len(trace) != 32 {
		t.Fatalf("trace id length = %d, want 32", len(trace))
	}
}

func TestLeaseClone(t *testing.T) {
	l := arcp.Lease{arcp.CapFSRead: {"/x/**"}}
	c := l.Clone()
	c[arcp.CapFSRead] = append(c[arcp.CapFSRead], "/y/**")
	if len(l[arcp.CapFSRead]) != 1 {
		t.Fatal("Clone did not deep copy")
	}
	if arcp.Lease(nil).Clone() != nil {
		t.Fatal("nil Lease must clone to nil")
	}
}

func TestLeaseJSONRoundtrip(t *testing.T) {
	in := arcp.Lease{arcp.CapNetFetch: {"https://api.example/*"}}
	body, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out arcp.Lease
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(err)
	}
	if len(out[arcp.CapNetFetch]) != 1 {
		t.Fatalf("decoded lease wrong: %v", out)
	}
}

// FuzzParseBudgetAmount must never panic on arbitrary input.
func FuzzParseBudgetAmount(f *testing.F) {
	for _, seed := range []string{"USD:1.00", "credits:1000", "USD:-1", "USD:", ":1.00", "", "::", "a:b"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, s string) {
		_, _ = arcp.ParseBudgetAmount(s)
	})
}

func TestRegisteredTypes(t *testing.T) {
	got := arcp.RegisteredTypes()
	if len(got) == 0 {
		t.Fatal("RegisteredTypes empty after messages init")
	}
}

func TestNewPayloadForType(t *testing.T) {
	if v := arcp.NewPayloadForType("session.hello"); v == nil {
		t.Fatal("session.hello should be registered")
	}
	if v := arcp.NewPayloadForType("not-a-real-type"); v != nil {
		t.Fatal("unknown type should return nil")
	}
}

func TestBudgetAmountString(t *testing.T) {
	b := arcp.BudgetAmount{Currency: "USD", Value: 1.5}
	if b.String() != "USD:1.5" {
		t.Fatalf("String() = %s", b.String())
	}
}

func TestBudgetAmountAcceptsLowercaseCurrency(t *testing.T) {
	// Vendor-defined lowercase currency, exercising the relaxed branch
	// in isCurrency.
	if _, err := arcp.ParseBudgetAmount("credits:100"); err != nil {
		t.Fatalf("credits accepted: %v", err)
	}
	if _, err := arcp.ParseBudgetAmount("coupon:1"); err != nil {
		t.Fatalf("coupon accepted: %v", err)
	}
}

func TestIsLeaseSubset(t *testing.T) {
	parent := arcp.Lease{
		arcp.CapFSRead:     {"/repo/**"},
		arcp.CapCostBudget: {"USD:10.0"},
	}
	child := arcp.Lease{
		arcp.CapFSRead:     {"/repo/foo/*"},
		arcp.CapCostBudget: {"USD:1.0"},
	}
	parentRem := map[arcp.Currency]float64{"USD": 10}
	if err := arcp.IsLeaseSubset(parent, child, parentRem, nil, nil); err != nil {
		t.Fatalf("valid subset failed: %v", err)
	}

	// Too-broad child path is rejected.
	badChild := arcp.Lease{arcp.CapFSRead: {"/other/**"}}
	if err := arcp.IsLeaseSubset(parent, badChild, parentRem, nil, nil); err == nil {
		t.Fatal("expected subset violation for /other/**")
	}

	// Over-budget child is rejected.
	overChild := arcp.Lease{arcp.CapCostBudget: {"USD:20.0"}}
	if err := arcp.IsLeaseSubset(parent, overChild, parentRem, nil, nil); err == nil {
		t.Fatal("expected subset violation for over-budget child")
	}

	// Currency not in parent.
	wrongCur := arcp.Lease{arcp.CapCostBudget: {"EUR:1.0"}}
	if err := arcp.IsLeaseSubset(parent, wrongCur, parentRem, nil, nil); err == nil {
		t.Fatal("expected subset violation for EUR child against USD parent")
	}

	// Capability missing from parent.
	missingCap := arcp.Lease{arcp.CapModelUse: {"tier-fast/*"}}
	if err := arcp.IsLeaseSubset(parent, missingCap, parentRem, nil, nil); err == nil {
		t.Fatal("expected subset violation for missing capability")
	}
}

// TestWithDetailsDoesNotMutateReceiver covers #59: WithDetails must not
// alias/mutate the receiver's Details map (the sentinels are shared).
func TestWithDetailsDoesNotMutateReceiver(t *testing.T) {
	base := arcp.ErrInvalidRequest.WithDetails(map[string]any{"a": 1})
	derived := base.WithDetails(map[string]any{"b": 2})
	if _, ok := base.Details["b"]; ok {
		t.Fatal("WithDetails mutated the receiver's Details map")
	}
	if _, ok := derived.Details["a"]; !ok {
		t.Fatal("derived error lost inherited detail 'a'")
	}
	if _, ok := derived.Details["b"]; !ok {
		t.Fatal("derived error missing new detail 'b'")
	}
}

// TestIsLeaseSubsetWildcardWidening covers #147: a child wildcard that
// widens authority beyond the parent must be rejected even though the
// child's pattern string glob-matches the parent.
func TestIsLeaseSubsetWildcardWidening(t *testing.T) {
	parent := arcp.Lease{arcp.CapFSRead: {"/data/*"}}
	widen := arcp.Lease{arcp.CapFSRead: {"/data/**"}}
	if err := arcp.IsLeaseSubset(parent, widen, nil, nil, nil); err == nil {
		t.Fatal("expected LEASE_SUBSET_VIOLATION: /data/* must not cover /data/**")
	}
	// Inverse: parent ** legitimately covers child *.
	wider := arcp.Lease{arcp.CapFSRead: {"/data/**"}}
	narrow := arcp.Lease{arcp.CapFSRead: {"/data/*"}}
	if err := arcp.IsLeaseSubset(wider, narrow, nil, nil, nil); err != nil {
		t.Fatalf("parent /data/** must cover child /data/*, got %v", err)
	}
}
