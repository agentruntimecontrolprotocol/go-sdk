// Package integration audit_test.go covers the 2026-06-11 deep-audit
// batch (issues #59-#160). Each test pins one contract the fix
// established.
package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
	"github.com/agentruntimecontrolprotocol/go-sdk/client"
	"github.com/agentruntimecontrolprotocol/go-sdk/messages"
	"github.com/agentruntimecontrolprotocol/go-sdk/server"
	"github.com/agentruntimecontrolprotocol/go-sdk/transport"
	"github.com/stretchr/testify/require"
)

// syncBuffer is a goroutine-safe io.Writer for capturing slog output.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

// TestPanicStackNotLeakedToClient (#160) asserts a panicking agent
// yields an INTERNAL_ERROR job.error with no stack/file-path content,
// while the full stack is recorded by the configured Logger.
func TestPanicStackNotLeakedToClient(t *testing.T) {
	sb := &syncBuffer{}
	logger := slog.New(slog.NewTextHandler(sb, &slog.HandlerOptions{Level: slog.LevelDebug}))
	srv := server.New(server.Options{Logger: logger})
	defer srv.Close()
	srv.RegisterAgent("boom", func(ctx context.Context, _ json.RawMessage, jc *server.JobContext) (any, error) {
		panic("kaboom-secret-detail")
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
	h, err := cli.Submit(ctx, client.SubmitRequest{Agent: "boom"})
	require.NoError(t, err)

	_, werr := h.Wait(ctx)
	require.Error(t, werr)
	var aerr *arcp.Error
	require.ErrorAs(t, werr, &aerr)
	require.Equal(t, arcp.CodeInternalError, aerr.Code)
	require.NotContains(t, aerr.Message, "goroutine")
	require.NotContains(t, aerr.Message, ".go:")
	require.NotContains(t, aerr.Message, "kaboom-secret-detail")

	// The server logger must have captured the stack.
	logged := sb.String()
	require.Contains(t, logged, "agent panicked")
	require.Contains(t, logged, "goroutine")
}

// TestServerHeartbeatNotStarvedByInboundTraffic (#159) keeps a steady
// stream of inbound session.ack envelopes flowing while the server has
// nothing to send, and asserts the server still emits session.ping in
// the server->client direction.
func TestServerHeartbeatNotStarvedByInboundTraffic(t *testing.T) {
	srv := server.New(server.Options{HeartbeatInterval: 50 * time.Millisecond})
	defer srv.Close()
	a, b := transport.NewMemoryPair()
	srvCtx, cancelSrv := context.WithCancel(context.Background())
	defer cancelSrv()
	go func() { _ = srv.Accept(srvCtx, b) }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	hello, _ := arcp.NewEnvelope(messages.TypeSessionHello, &messages.SessionHello{
		Client:       messages.ClientInfo{Name: "t"},
		Auth:         messages.AuthInfo{Token: "x"},
		Capabilities: messages.HelloCapabilities{Features: []string{"heartbeat", "ack"}},
	})
	require.NoError(t, a.Send(ctx, hello))
	welcome, err := a.Recv(ctx)
	require.NoError(t, err)
	require.Equal(t, messages.TypeSessionWelcome, welcome.Type)

	// Continuously send inbound acks (no server response) so the old
	// inbound-reset behaviour would suppress the outbound ping forever.
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		tick := time.NewTicker(10 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-stop:
				return
			case <-tick.C:
				ack, _ := arcp.NewEnvelope(messages.TypeSessionAck, &messages.SessionAck{LastProcessedSeq: 0})
				ack.SessionID = welcome.SessionID
				_ = a.Send(ctx, ack)
			}
		}
	}()

	gotPing := false
	for i := 0; i < 50 && !gotPing; i++ {
		env, err := a.Recv(ctx)
		require.NoError(t, err)
		if env.Type == messages.TypeSessionPing {
			gotPing = true
		}
	}
	require.True(t, gotPing, "server must emit session.ping despite constant inbound traffic")
}

// TestMalformedBudgetRejected (#149) submits a job whose lease_request
// carries a malformed cost.budget pattern and asserts the server
// rejects it with INVALID_REQUEST and never accepts the job.
func TestMalformedBudgetRejected(t *testing.T) {
	srv := server.New(server.Options{})
	defer srv.Close()
	started := make(chan struct{}, 1)
	srv.RegisterAgent("noop", func(ctx context.Context, _ json.RawMessage, jc *server.JobContext) (any, error) {
		started <- struct{}{}
		return nil, nil
	})
	a, b := transport.NewMemoryPair()
	srvCtx, cancelSrv := context.WithCancel(context.Background())
	defer cancelSrv()
	go func() { _ = srv.Accept(srvCtx, b) }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	hello, _ := arcp.NewEnvelope(messages.TypeSessionHello, &messages.SessionHello{
		Client: messages.ClientInfo{Name: "t"},
		Auth:   messages.AuthInfo{Token: "x"},
	})
	require.NoError(t, a.Send(ctx, hello))
	welcome, err := a.Recv(ctx)
	require.NoError(t, err)

	submit, _ := arcp.NewEnvelope(messages.TypeJobSubmit, &messages.JobSubmit{
		Agent:        "noop",
		LeaseRequest: arcp.Lease{arcp.CapCostBudget: {"USD 5.00"}},
	})
	submit.SessionID = welcome.SessionID
	require.NoError(t, a.Send(ctx, submit))

	resp, err := a.Recv(ctx)
	require.NoError(t, err)
	require.Equal(t, messages.TypeSessionError, resp.Type)
	var serr messages.SessionError
	require.NoError(t, resp.DecodePayload(&serr))
	require.Equal(t, arcp.CodeInvalidRequest, serr.Code)

	select {
	case <-started:
		t.Fatal("agent must not start for a rejected submit")
	case <-time.After(150 * time.Millisecond):
	}
	srv.Close()
}

// helper to keep imports tidy across the audit test file.
var _ = strings.Contains
