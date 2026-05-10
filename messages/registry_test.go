package messages_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/fizzpop/arcp-go"
	"github.com/fizzpop/arcp-go/messages"
)

// TestAllRegisteredTypesRoundTrip walks every registered type and
// confirms it round-trips through Envelope marshal/unmarshal.
func TestAllRegisteredTypesRoundTrip(t *testing.T) {
	t.Parallel()
	for _, typeName := range arcp.RegisteredMessageTypes() {
		t.Run(typeName, func(t *testing.T) {
			t.Parallel()
			env := arcp.Envelope{
				ID:        arcp.NewMessageID(),
				Timestamp: time.Now().UTC().Truncate(time.Second),
				Payload:   instantiate(t, typeName),
			}
			data, err := json.Marshal(env)
			if err != nil {
				t.Fatalf("marshal %s: %v", typeName, err)
			}
			var out arcp.Envelope
			if err := json.Unmarshal(data, &out); err != nil {
				t.Fatalf("unmarshal %s: %v", typeName, err)
			}
			if out.Type() != typeName {
				t.Errorf("round-trip type mismatch: got %q, want %q", out.Type(), typeName)
			}
		})
	}
}

func TestSpecificTypePayloadRoundTrip(t *testing.T) {
	t.Parallel()
	in := arcp.Envelope{
		ID:        arcp.NewMessageID(),
		Timestamp: time.Date(2026, 5, 9, 13, 0, 0, 0, time.UTC),
		Payload: &messages.SessionOpen{
			Auth: messages.Auth{
				Scheme: messages.AuthSchemeBearer,
				Token:  "secret",
			},
			Client: messages.ClientIdentity{
				Kind:    "claude-code",
				Version: "1.4.2",
			},
			Capabilities: messages.Capabilities{
				Streaming:  true,
				HumanInput: true,
				Extensions: []string{"arcpx.example.v1"},
			},
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
	got, ok := out.Payload.(*messages.SessionOpen)
	if !ok {
		t.Fatalf("payload type = %T, want *messages.SessionOpen", out.Payload)
	}
	if got.Auth.Scheme != messages.AuthSchemeBearer || got.Auth.Token != "secret" {
		t.Errorf("auth round-trip mismatch: %+v", got.Auth)
	}
	if !got.Capabilities.Streaming || !got.Capabilities.HumanInput {
		t.Errorf("capabilities lost: %+v", got.Capabilities)
	}
	if len(got.Capabilities.Extensions) != 1 || got.Capabilities.Extensions[0] != "arcpx.example.v1" {
		t.Errorf("extensions lost: %+v", got.Capabilities.Extensions)
	}
}

func TestErrorPayloadConversion(t *testing.T) {
	t.Parallel()
	src := arcp.NewError(arcp.CodePermissionDenied, "no").
		WithDetails(map[string]any{"resource": "x"})
	pl := messages.FromArcpError(src)
	if pl.Code != arcp.CodePermissionDenied {
		t.Errorf("code lost: %q", pl.Code)
	}
	round := pl.AsArcpError()
	if round.Code != src.Code || round.Message != src.Message {
		t.Errorf("round-trip mismatch: %+v vs %+v", round, src)
	}
	// nil should produce CodeOK.
	if got := messages.FromArcpError(nil); got.Code != arcp.CodeOK {
		t.Errorf("nil error -> %q, want OK", got.Code)
	}
}

func instantiate(t *testing.T, typeName string) arcp.MessageType {
	t.Helper()
	// Round-trip through the registry: encode an empty envelope of
	// this type to JSON, then decode it. The decoded envelope's
	// Payload is a fresh *T from the registry.
	wire := []byte(`{"arcp":"1.0","id":"msg_x","type":` + jsonString(typeName) +
		`,"timestamp":"2026-01-01T00:00:00Z","payload":{}}`)
	var env arcp.Envelope
	if err := json.Unmarshal(wire, &env); err != nil {
		t.Fatalf("instantiate %s via registry: %v", typeName, err)
	}
	return env.Payload
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
