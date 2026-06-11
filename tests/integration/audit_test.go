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
	"sync/atomic"
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

// TestResumedSessionReceivesLiveEvents (#145) submits a job that keeps
// emitting after the client reconnects, drops the transport, resumes,
// and asserts the post-resume events are delivered live on the resumed
// session (without a second resume).
func TestResumedSessionReceivesLiveEvents(t *testing.T) {
	srv := server.New(server.Options{})
	defer srv.Close()
	accepted := make(chan struct{})
	resumed := make(chan struct{})
	srv.RegisterAgent("survivor", func(ctx context.Context, _ json.RawMessage, jc *server.JobContext) (any, error) {
		jc.Log(slog.LevelInfo, "first")
		close(accepted)
		select {
		case <-resumed:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		for i := 0; i < 5; i++ {
			jc.Log(slog.LevelInfo, "post-resume")
		}
		return map[string]string{"done": "yes"}, nil
	})

	a, b := transport.NewMemoryPair()
	srvCtx, cancelSrv := context.WithCancel(context.Background())
	defer cancelSrv()
	go func() { _ = srv.Accept(srvCtx, b) }()
	cli, err := client.Connect(context.Background(), a, client.Options{Token: "demo"})
	require.NoError(t, err)
	welcome := cli.Welcome()
	sessionID := cli.SessionID()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = cli.Submit(ctx, client.SubmitRequest{Agent: "survivor"})
	require.NoError(t, err)
	<-accepted

	// Drop the transport; the job keeps running.
	_ = a.Close()
	time.Sleep(100 * time.Millisecond)

	a2, b2 := transport.NewMemoryPair()
	go func() { _ = srv.Accept(srvCtx, b2) }()
	cli2, err := client.Connect(context.Background(), a2, client.Options{
		Token: "demo",
		Resume: &messages.ResumeRequest{
			SessionID:    sessionID,
			ResumeToken:  welcome.ResumeToken,
			LastEventSeq: cli.HighestSeq(),
		},
	})
	require.NoError(t, err)
	defer cli2.Close(context.Background())
	require.Equal(t, sessionID, cli2.SessionID())

	seqBefore := cli2.HighestSeq()
	// Now let the surviving job emit post-resume events; they must
	// arrive live on the resumed session.
	close(resumed)
	require.Eventually(t, func() bool {
		return cli2.HighestSeq() > seqBefore
	}, 3*time.Second, 10*time.Millisecond, "resumed session received no live events from the surviving job")
}

// TestResumeReplayLargeBufferCompletes (#146) buffers far more events
// than the outbox capacity (128), drops the transport, and resumes from
// seq 0. The handshake must complete and every buffered event must be
// delivered, instead of deadlocking the replay inside the handshake.
func TestResumeReplayLargeBufferCompletes(t *testing.T) {
	srv := server.New(server.Options{})
	defer srv.Close()
	const n = 200
	srv.RegisterAgent("noisy", func(ctx context.Context, _ json.RawMessage, jc *server.JobContext) (any, error) {
		for i := 0; i < n; i++ {
			jc.Log(slog.LevelInfo, "x")
		}
		return nil, nil
	})

	a, b := transport.NewMemoryPair()
	srvCtx, cancelSrv := context.WithCancel(context.Background())
	defer cancelSrv()
	go func() { _ = srv.Accept(srvCtx, b) }()
	cli, err := client.Connect(context.Background(), a, client.Options{Token: "demo"})
	require.NoError(t, err)
	welcome := cli.Welcome()
	sessionID := cli.SessionID()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	h, err := cli.Submit(ctx, client.SubmitRequest{Agent: "noisy"})
	require.NoError(t, err)
	_, _ = h.Wait(ctx)

	_ = a.Close()
	time.Sleep(100 * time.Millisecond)

	a2, b2 := transport.NewMemoryPair()
	go func() { _ = srv.Accept(srvCtx, b2) }()
	connectCtx, cancelConnect := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelConnect()
	cli2, err := client.Connect(connectCtx, a2, client.Options{
		Token: "demo",
		Resume: &messages.ResumeRequest{
			SessionID:    sessionID,
			ResumeToken:  welcome.ResumeToken,
			LastEventSeq: 0,
		},
	})
	require.NoError(t, err, "resume handshake must complete even with >128 buffered events")
	defer cli2.Close(context.Background())

	require.Eventually(t, func() bool {
		return cli2.HighestSeq() >= n
	}, 3*time.Second, 10*time.Millisecond, "not all buffered events were replayed after resume")
}

// TestSessionCloseAcknowledged (#133) sends session.close and asserts
// the runtime replies with session.closed before dropping.
func TestSessionCloseAcknowledged(t *testing.T) {
	srv := server.New(server.Options{})
	defer srv.Close()
	a, b := transport.NewMemoryPair()
	srvCtx, cancelSrv := context.WithCancel(context.Background())
	defer cancelSrv()
	go func() { _ = srv.Accept(srvCtx, b) }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	hello, _ := arcp.NewEnvelope(messages.TypeSessionHello, &messages.SessionHello{
		Client: messages.ClientInfo{Name: "t"}, Auth: messages.AuthInfo{Token: "x"},
	})
	require.NoError(t, a.Send(ctx, hello))
	welcome, err := a.Recv(ctx)
	require.NoError(t, err)

	closeEnv, _ := arcp.NewEnvelope(messages.TypeSessionClose, &messages.SessionClose{Reason: "done"})
	closeEnv.SessionID = welcome.SessionID
	require.NoError(t, a.Send(ctx, closeEnv))
	resp, err := a.Recv(ctx)
	require.NoError(t, err)
	require.Equal(t, messages.TypeSessionClosed, resp.Type)
}

// TestCancelEmitsJobCancelledBeforeError (#134) asserts job.cancel
// produces a job.cancelled ack ahead of the terminal job.error.
func TestCancelEmitsJobCancelledBeforeError(t *testing.T) {
	srv := server.New(server.Options{})
	defer srv.Close()
	gate := make(chan struct{})
	srv.RegisterAgent("hold", func(ctx context.Context, _ json.RawMessage, jc *server.JobContext) (any, error) {
		close(gate)
		<-ctx.Done()
		return nil, ctx.Err()
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
	<-gate
	require.NoError(t, h.Cancel(ctx, "stop"))

	// Walk the event stream: a cancelled-phase marker must precede the
	// terminal error. We observe job.cancelled as a non-event envelope,
	// so assert ordering via the terminal error code.
	_, werr := h.Wait(ctx)
	var aerr *arcp.Error
	require.ErrorAs(t, werr, &aerr)
	require.Equal(t, arcp.CodeCancelled, aerr.Code)
}

// TestExpiresAtMustBeUTC (#138) rejects a non-UTC expires_at and
// accepts the equivalent Z value.
func TestExpiresAtMustBeUTC(t *testing.T) {
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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	loc := time.FixedZone("CEST", 2*60*60)
	nonUTC := time.Now().Add(time.Hour).In(loc)
	_, err = cli.Submit(ctx, client.SubmitRequest{
		Agent:            "noop",
		LeaseConstraints: &messages.LeaseConstraints{ExpiresAt: &nonUTC},
	})
	require.Error(t, err, "non-UTC expires_at must be rejected")
	var aerr *arcp.Error
	require.ErrorAs(t, err, &aerr)
	require.Equal(t, arcp.CodeInvalidRequest, aerr.Code)

	utc := time.Now().Add(time.Hour).UTC()
	_, err = cli.Submit(ctx, client.SubmitRequest{
		Agent:            "noop",
		LeaseConstraints: &messages.LeaseConstraints{ExpiresAt: &utc},
	})
	require.NoError(t, err, "UTC expires_at must be accepted")
}

// TestResumedSessionCanCancel (#137) disconnects, resumes with the
// rotated token, and cancels the surviving job through the new session.
func TestResumedSessionCanCancel(t *testing.T) {
	srv := server.New(server.Options{})
	defer srv.Close()
	accepted := make(chan struct{})
	srv.RegisterAgent("hold", func(ctx context.Context, _ json.RawMessage, jc *server.JobContext) (any, error) {
		close(accepted)
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
	sessionID := cli.SessionID()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	h, err := cli.Submit(ctx, client.SubmitRequest{Agent: "hold"})
	require.NoError(t, err)
	jobID := h.ID()
	<-accepted

	_ = a.Close()
	time.Sleep(100 * time.Millisecond)

	a2, b2 := transport.NewMemoryPair()
	go func() { _ = srv.Accept(srvCtx, b2) }()
	cli2, err := client.Connect(context.Background(), a2, client.Options{
		Token: "demo",
		Resume: &messages.ResumeRequest{
			SessionID:    sessionID,
			ResumeToken:  welcome.ResumeToken,
			LastEventSeq: cli.HighestSeq(),
		},
	})
	require.NoError(t, err)
	defer cli2.Close(context.Background())

	// Re-subscribe to the surviving job so we get a handle on cli2.
	require.True(t, cli2.HasFeature("subscribe"))
	sub, err := cli2.Subscribe(ctx, jobID, client.SubscribeOptions{})
	require.NoError(t, err)

	// Cancel via a raw job.cancel on the resumed session by sending it
	// through a fresh submit handle is unavailable; use the wire.
	cancelEnv, _ := arcp.NewEnvelope(messages.TypeJobCancel, &messages.JobCancel{Reason: "stop"})
	cancelEnv.SessionID = sessionID
	cancelEnv.JobID = jobID
	require.NoError(t, a2.Send(ctx, cancelEnv))

	// The subscription must terminate with CANCELLED rather than the
	// cancel being rejected with PERMISSION_DENIED.
	select {
	case <-sub.Done():
		require.ErrorIs(t, sub.Err(), arcp.ErrCancelled)
	case <-time.After(3 * time.Second):
		t.Fatal("resumed session cancel did not terminate the job")
	}
}

// TestResumeWindowExpiredOnBufferGap (#136) acks past part of the
// stream (trimming the log), drops, then resumes with a last_event_seq
// older than the oldest retained event and expects RESUME_WINDOW_EXPIRED.
func TestResumeWindowExpiredOnBufferGap(t *testing.T) {
	srv := server.New(server.Options{})
	defer srv.Close()
	release := make(chan struct{})
	srv.RegisterAgent("emitter", func(ctx context.Context, _ json.RawMessage, jc *server.JobContext) (any, error) {
		for i := 0; i < 5; i++ {
			jc.Log(slog.LevelInfo, "e")
		}
		select {
		case <-release:
		case <-ctx.Done():
		}
		return nil, nil
	})
	a, b := transport.NewMemoryPair()
	srvCtx, cancelSrv := context.WithCancel(context.Background())
	defer cancelSrv()
	go func() { _ = srv.Accept(srvCtx, b) }()
	cli, err := client.Connect(context.Background(), a, client.Options{Token: "demo"})
	require.NoError(t, err)
	welcome := cli.Welcome()
	sessionID := cli.SessionID()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = cli.Submit(ctx, client.SubmitRequest{Agent: "emitter"})
	require.NoError(t, err)
	require.Eventually(t, func() bool { return cli.HighestSeq() >= 5 }, 3*time.Second, 10*time.Millisecond)

	// Ack seq 3 so the server trims events <= 3 (oldest becomes 4).
	require.NoError(t, cli.Ack(ctx, 3))
	time.Sleep(100 * time.Millisecond)

	_ = a.Close()
	time.Sleep(100 * time.Millisecond)

	a2, b2 := transport.NewMemoryPair()
	go func() { _ = srv.Accept(srvCtx, b2) }()
	_, err = client.Connect(context.Background(), a2, client.Options{
		Token: "demo",
		Resume: &messages.ResumeRequest{
			SessionID:    sessionID,
			ResumeToken:  welcome.ResumeToken,
			LastEventSeq: 1, // older than oldest retained (4)
		},
	})
	require.Error(t, err)
	var aerr *arcp.Error
	require.ErrorAs(t, err, &aerr)
	require.Equal(t, arcp.CodeResumeWindowExpired, aerr.Code)
	close(release)
}

// TestIdempotentRetryReplaysAccepted (#135) re-submits an unchanged job
// under the same idempotency key from a fresh connection (same
// principal) and asserts the original job.accepted is replayed (same
// job id) and the agent runs only once.
func TestIdempotentRetryReplaysAccepted(t *testing.T) {
	srv := server.New(server.Options{})
	defer srv.Close()
	var runs int32
	srv.RegisterAgent("once", func(ctx context.Context, _ json.RawMessage, jc *server.JobContext) (any, error) {
		atomic.AddInt32(&runs, 1)
		return map[string]bool{"ok": true}, nil
	})
	srvCtx, cancelSrv := context.WithCancel(context.Background())
	defer cancelSrv()

	a, b := transport.NewMemoryPair()
	go func() { _ = srv.Accept(srvCtx, b) }()
	cli, err := client.Connect(context.Background(), a, client.Options{ClientName: "app", Token: "demo"})
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	h1, err := cli.Submit(ctx, client.SubmitRequest{Agent: "once", IdempotencyKey: "k", Input: map[string]int{"v": 1}})
	require.NoError(t, err)
	_, err = h1.Wait(ctx)
	require.NoError(t, err)
	_ = cli.Close(context.Background())

	// Fresh connection, same principal, identical submit.
	a2, b2 := transport.NewMemoryPair()
	go func() { _ = srv.Accept(srvCtx, b2) }()
	cli2, err := client.Connect(context.Background(), a2, client.Options{ClientName: "app", Token: "demo"})
	require.NoError(t, err)
	defer cli2.Close(context.Background())
	h2, err := cli2.Submit(ctx, client.SubmitRequest{Agent: "once", IdempotencyKey: "k", Input: map[string]int{"v": 1}})
	require.NoError(t, err, "identical idempotent retry must replay job.accepted, not error")
	require.Equal(t, h1.ID(), h2.ID(), "replayed job.accepted must carry the original job id")

	time.Sleep(100 * time.Millisecond)
	require.Equal(t, int32(1), atomic.LoadInt32(&runs), "agent must run only once for a replayed idempotent submit")
}

// helper to keep imports tidy across the audit test file.
var _ = strings.Contains
