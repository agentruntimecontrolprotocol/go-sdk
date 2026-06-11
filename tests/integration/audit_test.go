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

// TestPerRequestErrorDoesNotKillOtherJobs (#150) runs two jobs and
// triggers a per-request session.error (a denied subscribe to a missing
// job) on the same session, asserting both real jobs keep running
// rather than being torn down by failAll.
func TestPerRequestErrorDoesNotKillOtherJobs(t *testing.T) {
	srv := server.New(server.Options{})
	defer srv.Close()
	release := make(chan struct{})
	srv.RegisterAgent("hold", func(ctx context.Context, _ json.RawMessage, jc *server.JobContext) (any, error) {
		select {
		case <-release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		return map[string]string{"ok": "yes"}, nil
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
	h1, err := cli.Submit(ctx, client.SubmitRequest{Agent: "hold"})
	require.NoError(t, err)
	h2, err := cli.Submit(ctx, client.SubmitRequest{Agent: "hold"})
	require.NoError(t, err)

	// Subscribe to a missing job: the server replies session.error
	// (JOB_NOT_FOUND). Pre-fix this called failAll and killed h1/h2.
	_, serr := cli.Subscribe(ctx, "job-does-not-exist", client.SubscribeOptions{})
	require.Error(t, serr)

	time.Sleep(200 * time.Millisecond)
	select {
	case <-h1.Done():
		t.Fatal("h1 was terminated by an unrelated per-request error")
	default:
	}
	select {
	case <-h2.Done():
		t.Fatal("h2 was terminated by an unrelated per-request error")
	default:
	}
	close(release)
	_, err = h1.Wait(ctx)
	require.NoError(t, err)
	_, err = h2.Wait(ctx)
	require.NoError(t, err)
}

// TestSubscribeDeniedReturnsPromptly (#151) subscribes to a job that
// does not exist with a non-cancellable context and asserts Subscribe
// returns an error promptly instead of hanging forever.
func TestSubscribeDeniedReturnsPromptly(t *testing.T) {
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

	done := make(chan error, 1)
	go func() {
		// context.Background() — no deadline; must still return.
		_, serr := cli.Subscribe(context.Background(), "missing-job", client.SubscribeOptions{})
		done <- serr
	}()
	select {
	case serr := <-done:
		require.Error(t, serr)
	case <-time.After(3 * time.Second):
		t.Fatal("Subscribe to a missing job hung instead of returning an error")
	}
}

// TestListJobsUnblocksOnTransportFailure (#152) keeps a ListJobs call
// in flight (the peer deliberately never answers it) and then breaks
// the transport, asserting the call returns an error via failAll rather
// than hanging until its own (here, background) deadline.
func TestListJobsUnblocksOnTransportFailure(t *testing.T) {
	a, b := transport.NewMemoryPair()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	gotList := make(chan struct{})
	go func() {
		// Minimal fake runtime: accept hello, send welcome advertising
		// list_jobs, consume the list request without replying, then
		// drop the transport.
		if _, err := b.Recv(ctx); err != nil {
			return
		}
		welcome, _ := arcp.NewEnvelope(messages.TypeSessionWelcome, &messages.SessionWelcome{
			Capabilities: messages.WelcomeCapabilities{Features: []string{"list_jobs"}},
		})
		welcome.SessionID = "sess-1"
		_ = b.Send(ctx, welcome)
		if _, err := b.Recv(ctx); err != nil { // the list_jobs request
			return
		}
		close(gotList)
		_ = b.Close()
	}()

	cli, err := client.Connect(ctx, a, client.Options{Token: "demo", Features: []string{"list_jobs"}})
	require.NoError(t, err)
	require.True(t, cli.HasFeature("list_jobs"))

	done := make(chan error, 1)
	go func() {
		_, lerr := cli.ListJobs(context.Background(), client.ListJobsRequest{})
		done <- lerr
	}()
	<-gotList
	select {
	case lerr := <-done:
		require.Error(t, lerr)
	case <-time.After(3 * time.Second):
		t.Fatal("ListJobs hung after transport failure")
	}
}

// helper to keep imports tidy across the audit test file.
var _ = strings.Contains
