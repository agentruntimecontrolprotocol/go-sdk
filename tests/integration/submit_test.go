package integration_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
	"github.com/agentruntimecontrolprotocol/go-sdk/client"
	"github.com/agentruntimecontrolprotocol/go-sdk/messages"
	"github.com/agentruntimecontrolprotocol/go-sdk/server"
	"github.com/agentruntimecontrolprotocol/go-sdk/transport"
	"github.com/stretchr/testify/require"
)

func newPair(t *testing.T) (*server.Server, *client.Client, func()) {
	t.Helper()
	srv := server.New(server.Options{})
	srv.RegisterAgent("echo", func(ctx context.Context, input json.RawMessage, jc *server.JobContext) (any, error) {
		jc.Status("running", "")
		return map[string]json.RawMessage{"echo": input}, nil
	})
	a, b := transport.NewMemoryPair()
	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = srv.Accept(ctx, b) }()
	cli, err := client.Connect(ctx, a, client.Options{Token: "demo"})
	require.NoError(t, err)
	return srv, cli, func() {
		_ = cli.Close(context.Background())
		_ = srv.Close()
		cancel()
	}
}

func TestSubmitAndWait(t *testing.T) {
	_, cli, cleanup := newPair(t)
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	h, err := cli.Submit(ctx, client.SubmitRequest{
		Agent: "echo",
		Input: map[string]string{"text": "hello"},
	})
	require.NoError(t, err)
	require.NotEmpty(t, h.ID())

	res, err := h.Wait(ctx)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, messages.StatusSuccess, res.FinalStatus)
}

func TestFeatureNegotiation(t *testing.T) {
	_, cli, cleanup := newPair(t)
	defer cleanup()
	require.True(t, cli.HasFeature("heartbeat"))
	require.True(t, cli.HasFeature("subscribe"))
	require.False(t, cli.HasFeature("unknown"))
	_ = arcp.SDKVersion
}
