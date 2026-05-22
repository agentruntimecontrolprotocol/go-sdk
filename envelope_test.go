package arcp_test

import (
	"encoding/json"
	"testing"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
	"github.com/agentruntimecontrolprotocol/go-sdk/messages"
	_ "github.com/agentruntimecontrolprotocol/go-sdk/messages"
	"github.com/stretchr/testify/require"
)

func TestEnvelopeRoundTrip(t *testing.T) {
	hello := messages.SessionHello{
		Client: messages.ClientInfo{Name: "test", Version: "0.1"},
		Auth:   messages.AuthInfo{Scheme: "bearer", Token: "abc"},
		Capabilities: messages.HelloCapabilities{
			Encodings: []string{"json"},
			Features:  []string{"heartbeat", "ack"},
		},
	}
	env, err := arcp.NewEnvelope(messages.TypeSessionHello, &hello)
	require.NoError(t, err)
	env.SessionID = "sess_test"

	body, err := json.Marshal(env)
	require.NoError(t, err)

	var got arcp.Envelope
	require.NoError(t, json.Unmarshal(body, &got))
	require.Equal(t, env.Type, got.Type)
	require.Equal(t, env.SessionID, got.SessionID)
	require.Equal(t, arcp.ProtocolVersion, got.ARCP)

	v := arcp.NewPayloadForType(got.Type)
	require.NotNil(t, v)
	require.NoError(t, json.Unmarshal(got.Payload, v))
}

func TestEnvelopeValidate(t *testing.T) {
	env := arcp.Envelope{}
	require.Error(t, env.Validate())
	env.ARCP = arcp.ProtocolVersion
	require.Error(t, env.Validate())
	env.ID = "x"
	require.Error(t, env.Validate())
	env.Type = "session.hello"
	require.NoError(t, env.Validate())
}

func TestIntersectFeatures(t *testing.T) {
	a := []string{"heartbeat", "ack", "subscribe"}
	b := []string{"subscribe", "ack", "progress"}
	got := arcp.IntersectFeatures(a, b)
	require.Equal(t, []string{"ack", "subscribe"}, got)
}

func TestCodeAndIsRetryable(t *testing.T) {
	require.Equal(t, arcp.CodeLeaseExpired, arcp.Code(arcp.ErrLeaseExpired))
	require.False(t, arcp.IsRetryable(arcp.ErrLeaseExpired))
	require.True(t, arcp.IsRetryable(arcp.ErrInternalError))
	require.False(t, arcp.IsRetryable(arcp.ErrBudgetExhausted))
}

func TestParseBudgetAmount(t *testing.T) {
	for _, tc := range []struct {
		in  string
		ok  bool
		cur arcp.Currency
		val float64
	}{
		{"USD:5.00", true, "USD", 5.00},
		{"credits:1000", true, "credits", 1000},
		{"USD:-1", false, "", 0},
		{"USD:", false, "", 0},
		{":1.00", false, "", 0},
	} {
		amt, err := arcp.ParseBudgetAmount(tc.in)
		if !tc.ok {
			require.Error(t, err, tc.in)
			continue
		}
		require.NoError(t, err, tc.in)
		require.Equal(t, tc.cur, amt.Currency)
		require.InDelta(t, tc.val, amt.Value, 0.0001)
	}
}
