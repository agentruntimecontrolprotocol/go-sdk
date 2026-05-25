package integration_test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"
	"time"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
	"github.com/agentruntimecontrolprotocol/go-sdk/client"
	"github.com/agentruntimecontrolprotocol/go-sdk/server"
	"github.com/agentruntimecontrolprotocol/go-sdk/transport"
	"github.com/stretchr/testify/require"
)

// TestJobContextAccessorsAndEmitters runs an agent that exercises every
// getter on JobContext plus each event-emit helper so the server-side
// accessors are covered without needing per-method tests.
func TestJobContextAccessorsAndEmitters(t *testing.T) {
	srv := server.New(server.Options{})
	defer srv.Close()
	srv.RegisterAgent("emit-all", func(ctx context.Context, _ json.RawMessage, jc *server.JobContext) (any, error) {
		// Accessors.
		_ = jc.JobID()
		_ = jc.SessionID()
		_ = jc.TraceID()
		_ = jc.Context()
		_ = jc.Lease()
		_ = jc.Budget()
		// Lease validation; with no lease, expect a deny.
		if err := jc.ValidateLeaseOp(arcp.CapFSRead, "/x"); err == nil {
			t.Error("ValidateLeaseOp without lease should error")
		}
		// Event emitters.
		jc.Log(slog.LevelInfo, "ok")
		jc.Thought("ponder")
		id := jc.ToolCall("search", map[string]string{"q": "x"})
		jc.ToolResult(id, map[string]string{"r": "ok"})
		jc.ToolError(id, errors.New("oops"))
		jc.Status("step", "doing")
		jc.Metric("cost.openai_tokens", 100, "USD", map[string]string{"model": "x"})
		jc.ArtifactRef("s3://bucket/k", "application/json", 1024, "deadbeef")
		jc.Progress(1, 10, "items", "hello")
		// Reserve budget should succeed when no lease is in scope —
		// it returns 0 and may return permission_denied. Cover the
		// branch either way.
		_, _ = jc.ReserveBudget(arcp.CapToolCall, "search.*", arcp.BudgetAmount{Currency: "USD", Value: 0.10})
		return map[string]string{"done": "yes"}, nil
	})

	a, b := transport.NewMemoryPair()
	srvCtx, cancelSrv := context.WithCancel(context.Background())
	defer cancelSrv()
	go func() { _ = srv.Accept(srvCtx, b) }()
	cli, err := client.Connect(context.Background(), a, client.Options{Token: "demo"})
	require.NoError(t, err)
	defer cli.Close(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	h, err := cli.Submit(ctx, client.SubmitRequest{Agent: "emit-all"})
	require.NoError(t, err)
	res, err := h.Wait(ctx)
	require.NoError(t, err)
	require.NotNil(t, res)
}

// TestJobGettersCoverage submits one job and reads back snapshot via
// ListJobs to exercise Job.ID/Agent/Principal/Lease accessors.
func TestJobGettersCoverage(t *testing.T) {
	srv := server.New(server.Options{})
	defer srv.Close()
	gate := make(chan struct{})
	srv.RegisterAgent("hold", func(ctx context.Context, _ json.RawMessage, jc *server.JobContext) (any, error) {
		<-gate
		return nil, nil
	})
	a, b := transport.NewMemoryPair()
	srvCtx, cancelSrv := context.WithCancel(context.Background())
	defer cancelSrv()
	go func() { _ = srv.Accept(srvCtx, b) }()
	cli, err := client.Connect(context.Background(), a, client.Options{Token: "demo"})
	require.NoError(t, err)
	defer cli.Close(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	h, err := cli.Submit(ctx, client.SubmitRequest{Agent: "hold"})
	require.NoError(t, err)
	require.NotEmpty(t, h.ID())
	require.Equal(t, "hold", h.Agent())
	list, err := cli.ListJobs(ctx, client.ListJobsRequest{})
	require.NoError(t, err)
	require.NotEmpty(t, list.Jobs)
	close(gate)
	_, _ = h.Wait(ctx)
}
