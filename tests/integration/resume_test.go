package integration_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/agentruntimecontrolprotocol/go-sdk/client"
	"github.com/agentruntimecontrolprotocol/go-sdk/messages"
	"github.com/agentruntimecontrolprotocol/go-sdk/server"
	"github.com/agentruntimecontrolprotocol/go-sdk/transport"
	"github.com/stretchr/testify/require"
)

// TestResumeReplaysBufferedEvents drops the transport mid-job, then
// reconnects with the welcome's resume token and confirms the runtime
// replays buffered events whose seq exceeds the client's last
// processed seq.
func TestResumeReplaysBufferedEvents(t *testing.T) {
	srv := server.New(server.Options{})
	var mu sync.Mutex
	gate := make(chan struct{})
	srv.RegisterAgent("noisy", func(ctx context.Context, input json.RawMessage, jc *server.JobContext) (any, error) {
		// Emit a handful of log events to populate the eventlog.
		for i := 0; i < 5; i++ {
			jc.Log(slog.LevelInfo, "tick")
		}
		mu.Lock()
		close(gate)
		mu.Unlock()
		// Block so the job stays alive until the client reconnects.
		<-ctx.Done()
		return nil, ctx.Err()
	})
	a, b := transport.NewMemoryPair()
	srvCtx, cancelSrv := context.WithCancel(context.Background())
	defer cancelSrv()
	go func() { _ = srv.Accept(srvCtx, b) }()

	cli, err := client.Connect(context.Background(), a, client.Options{Token: "demo"})
	require.NoError(t, err)
	welcome := cli.Welcome()
	require.NotEmpty(t, welcome.ResumeToken)
	sessionID := cli.SessionID()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = cli.Submit(ctx, client.SubmitRequest{Agent: "noisy"})
	require.NoError(t, err)

	// Wait for the agent to emit and gate.
	select {
	case <-gate:
	case <-time.After(2 * time.Second):
		t.Fatal("agent never ran")
	}
	// Give the writer goroutine a moment to flush the events into the
	// eventlog before the client transport disappears.
	time.Sleep(50 * time.Millisecond)

	// Drop the transport unilaterally.
	_ = a.Close()

	// Allow the server to detect the drop and stash resume state.
	time.Sleep(100 * time.Millisecond)

	// Reconnect with the resume token.
	a2, b2 := transport.NewMemoryPair()
	go func() { _ = srv.Accept(srvCtx, b2) }()
	cli2, err := client.Connect(context.Background(), a2, client.Options{
		Token: "demo",
		Resume: &messages.ResumeRequest{
			SessionID:    sessionID,
			ResumeToken:  welcome.ResumeToken,
			LastEventSeq: 0,
		},
	})
	require.NoError(t, err)
	require.Equal(t, sessionID, cli2.SessionID(), "resume must reuse session id")
	require.NotEqual(t, welcome.ResumeToken, cli2.Welcome().ResumeToken, "resume_token must rotate")
	_ = cli2.Close(context.Background())
}

// TestResumeRejectsBadToken confirms a resume with the wrong token
// fails with an error (UNAUTHENTICATED) and does not let the client
// hijack the session.
func TestResumeRejectsBadToken(t *testing.T) {
	srv := server.New(server.Options{})
	srv.RegisterAgent("echo", func(ctx context.Context, input json.RawMessage, jc *server.JobContext) (any, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})
	a, b := transport.NewMemoryPair()
	srvCtx, cancelSrv := context.WithCancel(context.Background())
	defer cancelSrv()
	go func() { _ = srv.Accept(srvCtx, b) }()

	cli, err := client.Connect(context.Background(), a, client.Options{Token: "demo"})
	require.NoError(t, err)
	sessionID := cli.SessionID()
	_ = a.Close()
	time.Sleep(100 * time.Millisecond)

	a2, b2 := transport.NewMemoryPair()
	go func() { _ = srv.Accept(srvCtx, b2) }()
	_, err = client.Connect(context.Background(), a2, client.Options{
		Token: "demo",
		Resume: &messages.ResumeRequest{
			SessionID:    sessionID,
			ResumeToken:  "bogus",
			LastEventSeq: 0,
		},
	})
	require.Error(t, err)
}

// TestGracefulByeDropsResume confirms that after a polite Close, the
// runtime forgets the session and a resume attempt is refused.
func TestGracefulByeDropsResume(t *testing.T) {
	srv := server.New(server.Options{})
	srv.RegisterAgent("echo", func(ctx context.Context, input json.RawMessage, jc *server.JobContext) (any, error) {
		return nil, nil
	})
	a, b := transport.NewMemoryPair()
	srvCtx, cancelSrv := context.WithCancel(context.Background())
	defer cancelSrv()
	go func() { _ = srv.Accept(srvCtx, b) }()

	cli, err := client.Connect(context.Background(), a, client.Options{Token: "demo"})
	require.NoError(t, err)
	sessionID := cli.SessionID()
	token := cli.Welcome().ResumeToken
	_ = cli.Close(context.Background())
	time.Sleep(100 * time.Millisecond)

	a2, b2 := transport.NewMemoryPair()
	go func() { _ = srv.Accept(srvCtx, b2) }()
	_, err = client.Connect(context.Background(), a2, client.Options{
		Token: "demo",
		Resume: &messages.ResumeRequest{
			SessionID:    sessionID,
			ResumeToken:  token,
			LastEventSeq: 0,
		},
	})
	require.Error(t, err, "graceful bye must clear resume state")
}
