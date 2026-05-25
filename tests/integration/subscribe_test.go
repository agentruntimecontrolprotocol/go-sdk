package integration_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
	"github.com/agentruntimecontrolprotocol/go-sdk/client"
	"github.com/agentruntimecontrolprotocol/go-sdk/server"
	"github.com/agentruntimecontrolprotocol/go-sdk/transport"
	"github.com/stretchr/testify/require"
)

// TestSubscriptionGettersAndEvents exercises the subscription getter
// surface and verifies push delivers events to a live subscriber.
func TestSubscriptionGettersAndEvents(t *testing.T) {
	srv := server.New(server.Options{})
	defer srv.Close()
	gate := make(chan struct{})
	srv.RegisterAgent("emitter", func(ctx context.Context, _ json.RawMessage, jc *server.JobContext) (any, error) {
		// Wait for the subscriber to attach, then emit so push() runs
		// against a live subscription.
		<-gate
		for i := 0; i < 3; i++ {
			jc.Log(slog.LevelInfo, "hello")
		}
		return nil, nil
	})

	a1, b1 := transport.NewMemoryPair()
	a2, b2 := transport.NewMemoryPair()
	srvCtx, cancelSrv := context.WithCancel(context.Background())
	defer cancelSrv()
	go func() { _ = srv.Accept(srvCtx, b1) }()
	go func() { _ = srv.Accept(srvCtx, b2) }()

	submitter, err := client.Connect(context.Background(), a1, client.Options{Token: "demo"})
	require.NoError(t, err)
	defer submitter.Close(context.Background())
	subscriber, err := client.Connect(context.Background(), a2, client.Options{Token: "demo"})
	require.NoError(t, err)
	defer subscriber.Close(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	h, err := submitter.Submit(ctx, client.SubmitRequest{Agent: "emitter"})
	require.NoError(t, err)
	require.NotEmpty(t, h.Agent())
	sub, err := subscriber.Subscribe(ctx, h.ID(), client.SubscribeOptions{})
	require.NoError(t, err)
	require.Equal(t, h.ID(), sub.JobID())
	require.NotEmpty(t, sub.CurrentStatus())
	require.NotEmpty(t, sub.Agent())
	require.Equal(t, "", sub.ParentJobID())
	require.NotEmpty(t, sub.TraceID())
	_ = sub.SubscribedFrom()
	require.False(t, sub.Replayed())
	// Lease may be nil on default; just access for coverage.
	_ = sub.Lease()
	// Now let the agent emit, after subscription is attached.
	close(gate)
	select {
	case ev := <-sub.Events():
		require.NotEmpty(t, ev.Kind)
	case <-time.After(2 * time.Second):
		t.Fatal("subscriber received no events")
	}
	require.NoError(t, sub.Close(context.Background()))
	select {
	case <-sub.Done():
	case <-time.After(time.Second):
		t.Fatal("subscription Done not signalled")
	}
	require.NoError(t, sub.Err())
	_, _ = h.Wait(ctx)
}

// TestClientAckAndListJobsAndFeatures exercises a few small client
// surface methods to lift coverage in client.go.
func TestClientAckAndListJobsAndFeatures(t *testing.T) {
	srv := server.New(server.Options{})
	defer srv.Close()
	srv.RegisterAgent("noop", func(ctx context.Context, _ json.RawMessage, jc *server.JobContext) (any, error) {
		return nil, nil
	})
	a, b := transport.NewMemoryPair()
	srvCtx, cancelSrv := context.WithCancel(context.Background())
	defer cancelSrv()
	go func() { _ = srv.Accept(srvCtx, b) }()

	cli, err := client.Connect(context.Background(), a, client.Options{Token: "demo"})
	require.NoError(t, err)
	defer cli.Close(context.Background())

	// Features() coverage.
	feats := cli.Features()
	require.NotEmpty(t, feats)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	h, err := cli.Submit(ctx, client.SubmitRequest{Agent: "noop"})
	require.NoError(t, err)
	_, _ = h.Wait(ctx)

	// Ack explicitly.
	if err := cli.Ack(ctx, cli.HighestSeq()); err != nil {
		t.Fatalf("Ack: %v", err)
	}

	list, err := cli.ListJobs(ctx, client.ListJobsRequest{})
	require.NoError(t, err)
	require.NotNil(t, list)
}

// TestAckRequiresFeature verifies the gating on Ack().
func TestAckRequiresFeature(t *testing.T) {
	srv := server.New(server.Options{Features: []string{"heartbeat"}})
	defer srv.Close()
	a, b := transport.NewMemoryPair()
	srvCtx, cancelSrv := context.WithCancel(context.Background())
	defer cancelSrv()
	go func() { _ = srv.Accept(srvCtx, b) }()
	cli, err := client.Connect(context.Background(), a, client.Options{Token: "demo"})
	require.NoError(t, err)
	defer cli.Close(context.Background())
	require.False(t, cli.HasFeature("ack"))
	err = cli.Ack(context.Background(), 1)
	require.Error(t, err)
	var aerr *arcp.Error
	require.ErrorAs(t, err, &aerr)
}

// TestListJobsRequiresFeature verifies the gate on ListJobs().
func TestListJobsRequiresFeature(t *testing.T) {
	srv := server.New(server.Options{Features: []string{"heartbeat"}})
	defer srv.Close()
	a, b := transport.NewMemoryPair()
	srvCtx, cancelSrv := context.WithCancel(context.Background())
	defer cancelSrv()
	go func() { _ = srv.Accept(srvCtx, b) }()
	cli, err := client.Connect(context.Background(), a, client.Options{Token: "demo"})
	require.NoError(t, err)
	defer cli.Close(context.Background())
	_, err = cli.ListJobs(context.Background(), client.ListJobsRequest{})
	require.Error(t, err)
}

// TestSubscribeRequiresFeature verifies the gate on Subscribe().
func TestSubscribeRequiresFeature(t *testing.T) {
	srv := server.New(server.Options{Features: []string{"heartbeat"}})
	defer srv.Close()
	a, b := transport.NewMemoryPair()
	srvCtx, cancelSrv := context.WithCancel(context.Background())
	defer cancelSrv()
	go func() { _ = srv.Accept(srvCtx, b) }()
	cli, err := client.Connect(context.Background(), a, client.Options{Token: "demo"})
	require.NoError(t, err)
	defer cli.Close(context.Background())
	_, err = cli.Subscribe(context.Background(), "any-job-id", client.SubscribeOptions{})
	require.Error(t, err)
}
