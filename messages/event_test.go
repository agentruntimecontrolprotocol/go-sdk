package messages

import (
	"encoding/json"
	"testing"
)

func TestToolResultValidate(t *testing.T) {
	// Both set → invalid.
	if err := (ToolResultBody{
		CallID: "c",
		Result: json.RawMessage(`{"x":1}`),
		Error:  &ToolError{Code: "X"},
	}).Validate(); err == nil {
		t.Fatal("expected validation error when both result and error are set")
	}
	// Neither set → invalid.
	if err := (ToolResultBody{CallID: "c"}).Validate(); err == nil {
		t.Fatal("expected validation error when neither result nor error is set")
	}
	// Only result → ok.
	if err := (ToolResultBody{CallID: "c", Result: json.RawMessage(`1`)}).Validate(); err != nil {
		t.Fatal(err)
	}
	// Only error → ok.
	if err := (ToolResultBody{CallID: "c", Error: &ToolError{Code: "X"}}).Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestProgressValidate(t *testing.T) {
	if err := (ProgressBody{Current: 5, Total: 0}).Validate(); err != nil {
		t.Fatal(err)
	}
	if err := (ProgressBody{Current: 5, Total: 10}).Validate(); err != nil {
		t.Fatal(err)
	}
	if err := (ProgressBody{Current: 11, Total: 10}).Validate(); err == nil {
		t.Fatal("current > total must be rejected")
	}
}

func TestResultChunkValidate(t *testing.T) {
	if err := (ResultChunkBody{}).Validate(); err == nil {
		t.Fatal("missing result_id must reject")
	}
	if err := (ResultChunkBody{ResultID: "r", Encoding: "weird"}).Validate(); err == nil {
		t.Fatal("invalid encoding must reject")
	}
	if err := (ResultChunkBody{ResultID: "r", Encoding: "utf8"}).Validate(); err != nil {
		t.Fatal(err)
	}
	if err := (ResultChunkBody{ResultID: "r", Encoding: "base64"}).Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestDecodeEventBody(t *testing.T) {
	body, _ := json.Marshal(LogBody{Level: "INFO", Message: "ok"})
	ev := &JobEvent{Kind: KindLog, Body: body}
	out, err := DecodeEventBody(ev)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := out.(*LogBody); !ok {
		t.Fatalf("decoded to %T, want *LogBody", out)
	}

	// Result chunk validation must surface through DecodeEventBody.
	bad, _ := json.Marshal(ResultChunkBody{Encoding: "utf8"})
	_, err = DecodeEventBody(&JobEvent{Kind: KindResultChunk, Body: bad})
	if err == nil {
		t.Fatal("expected DecodeEventBody to surface ResultChunkBody.Validate error")
	}

	// Progress validation.
	bad, _ = json.Marshal(ProgressBody{Current: 100, Total: 1})
	_, err = DecodeEventBody(&JobEvent{Kind: KindProgress, Body: bad})
	if err == nil {
		t.Fatal("expected ProgressBody.Validate error")
	}

	// Tool result validation.
	good, _ := json.Marshal(ToolResultBody{CallID: "c", Result: json.RawMessage(`1`)})
	if _, err := DecodeEventBody(&JobEvent{Kind: KindToolResult, Body: good}); err != nil {
		t.Fatal(err)
	}

	// Unknown kind returns raw body.
	raw := json.RawMessage(`{"x":"y"}`)
	out, err = DecodeEventBody(&JobEvent{Kind: "x-vendor.custom", Body: raw})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := out.(json.RawMessage); !ok {
		t.Fatalf("unknown kind returned %T, want raw", out)
	}
}

func TestNewEventBody(t *testing.T) {
	raw, err := NewEventBody(LogBody{Level: "INFO", Message: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) == 0 {
		t.Fatal("empty body")
	}
	// Marshal failure path.
	if _, err := NewEventBody(make(chan int)); err == nil {
		t.Fatal("expected marshal error for chan")
	}
}

func TestParseAgentRef(t *testing.T) {
	good := []struct {
		in      string
		name    string
		version string
	}{
		{"echo", "echo", ""},
		{"my.agent", "my.agent", ""},
		{"echo@1.0.0", "echo", "1.0.0"},
		{"echo@v1.0+build.5", "echo", "v1.0+build.5"},
	}
	for _, tc := range good {
		ref, err := ParseAgentRef(tc.in)
		if err != nil {
			t.Fatalf("%s: %v", tc.in, err)
		}
		if ref.Name != tc.name || ref.Version != tc.version {
			t.Fatalf("%s parsed as %+v", tc.in, ref)
		}
	}
	bad := []string{"", "Echo", "echo@", "echo@bad/version", "-leading", ".dot-first"}
	for _, in := range bad {
		if _, err := ParseAgentRef(in); err == nil {
			t.Fatalf("%q must be rejected", in)
		}
	}
}

func TestAgentRefString(t *testing.T) {
	if (AgentRef{Name: "n"}.String()) != "n" {
		t.Fatal()
	}
	if (AgentRef{Name: "n", Version: "1"}.String()) != "n@1" {
		t.Fatal()
	}
}

// FuzzParseAgentRef ensures ParseAgentRef never panics on arbitrary
// input.
func FuzzParseAgentRef(f *testing.F) {
	for _, seed := range []string{"", "echo", "echo@1", "@", "echo@bad/version", "Echo", "-leading"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, s string) {
		_, _ = ParseAgentRef(s) // must not panic
	})
}
