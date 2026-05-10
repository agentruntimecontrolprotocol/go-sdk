package arcp_test

import "github.com/fizzpop/arcp-go"

// Test-only message types used by envelope_test.go to exercise the
// dispatch registry without depending on the messages/ package, which
// is built out in Phase 2.

const (
	testPingType   = "test.ping"
	testPongType   = "test.pong"
	testNestedType = "test.nested"
)

type testPing struct {
	Greeting string `json:"greeting,omitempty"`
}

func (testPing) ARCPType() string { return testPingType }

type testPong struct {
	Echo string `json:"echo,omitempty"`
}

func (testPong) ARCPType() string { return testPongType }

type testNested struct {
	Title string         `json:"title"`
	Items []string       `json:"items,omitempty"`
	Meta  map[string]any `json:"meta,omitempty"`
}

func (testNested) ARCPType() string { return testNestedType }

func init() {
	arcp.RegisterMessageType(testPingType, func() arcp.MessageType { return &testPing{} })
	arcp.RegisterMessageType(testPongType, func() arcp.MessageType { return &testPong{} })
	arcp.RegisterMessageType(testNestedType, func() arcp.MessageType { return &testNested{} })
}
