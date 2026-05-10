package arcp_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fizzpop/arcp-go"
)

const testGreeting = "hello"

func TestEnvelopeRoundTrip(t *testing.T) {
	t.Parallel()
	in := arcp.Envelope{
		ID:             arcp.MessageID("msg_01TEST"),
		Timestamp:      time.Date(2026, 5, 9, 13, 0, 0, 0, time.UTC),
		SessionID:      arcp.SessionID("sess_01"),
		JobID:          arcp.JobID("job_01"),
		TraceID:        arcp.TraceID("trace_01"),
		SpanID:         arcp.SpanID("span_01"),
		CorrelationID:  arcp.MessageID("msg_orig"),
		IdempotencyKey: "refund-123",
		Priority:       arcp.PriorityHigh,
		Payload:        &testPing{Greeting: testGreeting},
	}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out arcp.Envelope
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.ID != in.ID {
		t.Errorf("ID mismatch: %q vs %q", out.ID, in.ID)
	}
	if out.SessionID != in.SessionID {
		t.Errorf("SessionID mismatch: %q vs %q", out.SessionID, in.SessionID)
	}
	if out.IdempotencyKey != in.IdempotencyKey {
		t.Errorf("IdempotencyKey mismatch: %q vs %q", out.IdempotencyKey, in.IdempotencyKey)
	}
	if out.Priority != in.Priority {
		t.Errorf("Priority mismatch: %q vs %q", out.Priority, in.Priority)
	}
	if !out.Timestamp.Equal(in.Timestamp) {
		t.Errorf("Timestamp mismatch: %v vs %v", out.Timestamp, in.Timestamp)
	}
	gotPayload, ok := out.Payload.(*testPing)
	if !ok {
		t.Fatalf("payload type = %T, want *testPing", out.Payload)
	}
	if gotPayload.Greeting != "hello" {
		t.Errorf("payload greeting = %q, want hello", gotPayload.Greeting)
	}
}

func TestEnvelopeMarshalSetsARCPVersion(t *testing.T) {
	t.Parallel()
	in := arcp.Envelope{
		ID:        arcp.MessageID("msg_01"),
		Timestamp: time.Date(2026, 5, 9, 13, 0, 0, 0, time.UTC),
		Payload:   &testPing{Greeting: "v"},
	}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal raw: %v", err)
	}
	if raw["arcp"] != arcp.ProtocolVersion {
		t.Errorf("arcp = %v, want %q", raw["arcp"], arcp.ProtocolVersion)
	}
	if raw["type"] != testPingType {
		t.Errorf("type = %v, want test.ping", raw["type"])
	}
}

func TestEnvelopeMarshalNilPayloadFails(t *testing.T) {
	t.Parallel()
	in := arcp.Envelope{ID: arcp.MessageID("msg_x")}
	if _, err := json.Marshal(in); err == nil {
		t.Errorf("marshal of nil-payload envelope should fail")
	}
}

func TestEnvelopeUnmarshalUnknownTypeReturnsUnimplemented(t *testing.T) {
	t.Parallel()
	wire := `{
		"arcp": "1.0",
		"id": "msg_01",
		"type": "totally.unknown.type",
		"timestamp": "2026-05-09T13:00:00Z",
		"payload": {}
	}`
	var env arcp.Envelope
	err := json.Unmarshal([]byte(wire), &env)
	if err == nil {
		t.Fatalf("expected error for unknown type")
	}
	if !errors.Is(err, arcp.ErrUnimplemented) {
		t.Errorf("expected ErrUnimplemented, got %v (code=%q)", err, arcp.Code(err))
	}
}

func TestEnvelopeUnmarshalMalformedPayload(t *testing.T) {
	t.Parallel()
	// test.ping declares Greeting as string. Pass a number.
	wire := `{
		"arcp": "1.0",
		"id": "msg_01",
		"type": "test.ping",
		"timestamp": "2026-05-09T13:00:00Z",
		"payload": {"greeting": 42}
	}`
	var env arcp.Envelope
	err := json.Unmarshal([]byte(wire), &env)
	if err == nil {
		t.Fatalf("expected decode error for bad payload type")
	}
	if !errors.Is(err, arcp.ErrInvalidArgument) {
		t.Errorf("expected ErrInvalidArgument, got %v", err)
	}
}

func TestUnmarshalEnvelopeHeader(t *testing.T) {
	t.Parallel()
	wire := `{
		"arcp": "1.0",
		"id": "msg_h",
		"type": "test.ping",
		"timestamp": "2026-05-09T13:00:00Z",
		"session_id": "sess_h",
		"trace_id": "trace_h",
		"priority": "high",
		"extensions": {"optional": true},
		"payload": {"greeting": "hi"}
	}`
	hdr, err := arcp.UnmarshalEnvelopeHeader([]byte(wire))
	if err != nil {
		t.Fatalf("UnmarshalEnvelopeHeader: %v", err)
	}
	if hdr.Type != testPingType || hdr.SessionID != "sess_h" || hdr.TraceID != "trace_h" {
		t.Errorf("header fields wrong: %+v", hdr)
	}
	if hdr.Priority != arcp.PriorityHigh {
		t.Errorf("priority = %q, want high", hdr.Priority)
	}
	if !hdr.Optional {
		t.Errorf("expected Optional=true from extensions")
	}
}

func TestEnvelopeTypeReturnsPayloadType(t *testing.T) {
	t.Parallel()
	e := arcp.Envelope{Payload: &testPing{}}
	if got := e.Type(); got != testPingType {
		t.Errorf("Type() = %q, want test.ping", got)
	}
	empty := arcp.Envelope{}
	if got := empty.Type(); got != "" {
		t.Errorf("empty Type() = %q, want empty", got)
	}
}

func TestRegisterMessageTypeDuplicatePanics(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic on duplicate registration")
		}
	}()
	arcp.RegisterMessageType(testPingType, func() arcp.MessageType { return &testPing{} })
}

func TestRegisterMessageTypeRejectsEmpty(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic on empty name")
		}
	}()
	arcp.RegisterMessageType("", func() arcp.MessageType { return &testPing{} })
}

func TestRegisterMessageTypeRejectsNilFactory(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic on nil factory")
		}
	}()
	arcp.RegisterMessageType("test.never", nil)
}

func TestRegisteredMessageTypesIncludesTestTypes(t *testing.T) {
	t.Parallel()
	got := arcp.RegisteredMessageTypes()
	have := func(name string) bool {
		for _, n := range got {
			if n == name {
				return true
			}
		}
		return false
	}
	for _, want := range []string{testPingType, "test.pong", "test.nested"} {
		if !have(want) {
			t.Errorf("RegisteredMessageTypes missing %q (got %v)", want, got)
		}
	}
}

func TestEnvelopeNestedPayloadRoundTrip(t *testing.T) {
	t.Parallel()
	in := arcp.Envelope{
		ID:        arcp.MessageID("msg_n"),
		Timestamp: time.Now().UTC().Truncate(time.Microsecond),
		Payload: &testNested{
			Title: "nested",
			Items: []string{"a", "b"},
			Meta:  map[string]any{"k": float64(7)},
		},
	}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out arcp.Envelope
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	got, ok := out.Payload.(*testNested)
	if !ok {
		t.Fatalf("payload type = %T", out.Payload)
	}
	if got.Title != "nested" || len(got.Items) != 2 || got.Items[0] != "a" || got.Items[1] != "b" {
		t.Errorf("payload mismatch: %+v", got)
	}
	if v, ok := got.Meta["k"]; !ok || v.(float64) != 7 {
		t.Errorf("payload meta mismatch: %+v", got.Meta)
	}
}

// TestEnvelopeGoldenSnapshot locks the wire format of a known envelope
// against testdata/golden/ping.json. To regenerate after an
// intentional wire-format change, run the test with -update.
func TestEnvelopeGoldenSnapshot(t *testing.T) {
	t.Parallel()
	in := arcp.Envelope{
		ID:        "msg_01ABC",
		Timestamp: time.Date(2026, 5, 9, 13, 0, 0, 0, time.UTC),
		SessionID: "sess_01",
		Priority:  arcp.PriorityNormal,
		Payload:   &testPing{Greeting: testGreeting},
	}
	got, err := json.MarshalIndent(in, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	goldenPath := filepath.Join("testdata", "golden", "ping.json")
	want, err := os.ReadFile(goldenPath) //#nosec G304 -- fixed test path under testdata/
	if err != nil {
		t.Fatalf("read golden %s: %v", goldenPath, err)
	}
	gotStr := strings.TrimSpace(string(got))
	wantStr := strings.TrimSpace(string(want))
	if gotStr != wantStr {
		t.Errorf("envelope wire format diverged from golden:\n--- got\n%s\n--- want\n%s", gotStr, wantStr)
	}
}
