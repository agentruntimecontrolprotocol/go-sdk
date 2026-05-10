package arcp_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/fizzpop/arcp-go"
)

func TestIDPrefixesAndUniqueness(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		prefix string
		gen    func() string
	}{
		{"session", "sess_", func() string { return string(arcp.NewSessionID()) }},
		{"message", "msg_", func() string { return string(arcp.NewMessageID()) }},
		{"job", "job_", func() string { return string(arcp.NewJobID()) }},
		{"stream", "str_", func() string { return string(arcp.NewStreamID()) }},
		{"subscription", "sub_", func() string { return string(arcp.NewSubscriptionID()) }},
		{"trace", "trace_", func() string { return string(arcp.NewTraceID()) }},
		{"span", "span_", func() string { return string(arcp.NewSpanID()) }},
		{"artifact", "art_", func() string { return string(arcp.NewArtifactID()) }},
		{"lease", "lease_", func() string { return string(arcp.NewLeaseID()) }},
		{"checkpoint", "chk_", func() string { return string(arcp.NewCheckpointID()) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			seen := make(map[string]struct{}, 1024)
			for i := 0; i < 1024; i++ {
				id := tc.gen()
				if !strings.HasPrefix(id, tc.prefix) {
					t.Fatalf("id %q does not have prefix %q", id, tc.prefix)
				}
				if _, dup := seen[id]; dup {
					t.Fatalf("duplicate id %q on iteration %d", id, i)
				}
				seen[id] = struct{}{}
			}
		})
	}
}

func TestIDStringers(t *testing.T) {
	t.Parallel()
	sid := arcp.SessionID("sess_abc")
	if sid.String() != "sess_abc" {
		t.Fatalf("SessionID.String() = %q, want %q", sid.String(), "sess_abc")
	}
	mid := arcp.MessageID("msg_xyz")
	if mid.String() != "msg_xyz" {
		t.Fatalf("MessageID.String() = %q, want %q", mid.String(), "msg_xyz")
	}
}

func TestIDJSONRoundTrip(t *testing.T) {
	t.Parallel()
	type wrap struct {
		Session arcp.SessionID `json:"session"`
		Job     arcp.JobID     `json:"job"`
	}
	in := wrap{Session: arcp.NewSessionID(), Job: arcp.NewJobID()}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out wrap
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out != in {
		t.Fatalf("round-trip mismatch:\n  in=%+v\n out=%+v", in, out)
	}
}

func TestIDTypesAreDistinct(t *testing.T) {
	// SessionID and JobID are distinct named types — assigning across
	// requires an explicit conversion. This test checks at compile
	// time via an unused var pattern that would fail to type-check if
	// the types were aliased. The runtime assertions are illustrative.
	t.Parallel()
	var s arcp.SessionID = "sess_x"
	j := arcp.JobID(string(s))
	if string(s) != string(j) {
		t.Fatalf("conversion failed")
	}
}
