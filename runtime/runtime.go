package runtime

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/fizzpop/arcp-go"
	"github.com/fizzpop/arcp-go/auth"
	"github.com/fizzpop/arcp-go/messages"
	"github.com/fizzpop/arcp-go/store"
	"github.com/fizzpop/arcp-go/transport"
)

// Options configures a Runtime.
type Options struct {
	// Auth is the credential verifier (RFC §8). Required.
	Auth auth.Verifier
	// Identity is what the runtime advertises in session.accepted
	// (RFC §8.3). Required.
	Identity messages.RuntimeIdentity
	// Capabilities are what the runtime is willing to support. The
	// negotiated set is the intersection with the client's offer.
	// (RFC §7.)
	Capabilities messages.Capabilities
	// EventLog is the optional persistent log used for resume and
	// subscription backfill. nil disables persistence.
	EventLog *store.EventLog
	// Logger receives diagnostic messages. nil defaults to
	// slog.Default().
	Logger *slog.Logger
}

// Runtime is the server-side ARCP entry point. One Runtime serves
// many Transports concurrently via Serve.
type Runtime struct {
	opts Options
}

// New constructs a Runtime. Returns INVALID_ARGUMENT if required
// options are missing.
func New(opts Options) (*Runtime, error) {
	if opts.Auth == nil {
		return nil, arcp.NewError(arcp.CodeInvalidArgument, "Runtime: Options.Auth is required")
	}
	if opts.Identity.Kind == "" {
		return nil, arcp.NewError(arcp.CodeInvalidArgument, "Runtime: Options.Identity.Kind is required")
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.Capabilities.HeartbeatIntervalSeconds == 0 {
		opts.Capabilities.HeartbeatIntervalSeconds = 30
	}
	if opts.Capabilities.HeartbeatRecovery == "" {
		opts.Capabilities.HeartbeatRecovery = "fail"
	}
	return &Runtime{opts: opts}, nil
}

// Serve drives a single transport through the four-step session
// handshake (RFC §8.1) and then runs the steady-state message loop
// until the session ends or ctx is cancelled.
//
// Serve blocks. Run it in a goroutine if you need to accept multiple
// transports concurrently.
func (r *Runtime) Serve(ctx context.Context, t transport.Transport) error {
	sess, err := r.handshake(ctx, t)
	if err != nil {
		return err
	}
	r.opts.Logger.Info("session accepted", "session_id", sess.ID, "principal", sess.Principal.Subject)
	return r.runLoop(ctx, t, sess)
}

// session captures per-session state.
type session struct {
	ID           arcp.SessionID
	Principal    auth.Principal
	Capabilities messages.Capabilities
}

// handshake implements RFC §8.1 (steps 1-4) for v0.1: bearer/none, no
// challenge round-trip. signed_jwt is supported for the client side
// when the verifier accepts the token without a challenge; full
// challenge/response for signed_jwt is out of scope for v0.1.
func (r *Runtime) handshake(ctx context.Context, t transport.Transport) (*session, error) {
	env, err := t.Recv(ctx)
	if err != nil {
		return nil, fmt.Errorf("handshake: receive session.open: %w", err)
	}
	open, ok := env.Payload.(*messages.SessionOpen)
	if !ok {
		_ = r.sendReject(ctx, t, env, arcp.CodeFailedPrecondition,
			"first message must be session.open")
		return nil, arcp.NewError(arcp.CodeFailedPrecondition,
			fmt.Sprintf("handshake: first message was %s, want session.open", env.Type()))
	}

	// Anonymous gating (RFC §4.6 / §8.2).
	if open.Auth.Scheme == messages.AuthSchemeNone && !r.opts.Capabilities.Anonymous {
		_ = r.sendUnauthenticated(ctx, t, env, arcp.CodeUnauthenticated,
			"anonymous capability not advertised by this runtime")
		return nil, arcp.ErrUnauthenticated.WithMessage("anonymous not negotiated")
	}

	principal, err := r.opts.Auth.Verify(ctx, open.Auth, open.Client)
	if err != nil {
		_ = r.sendUnauthenticated(ctx, t, env, arcp.Code(err), err.Error())
		return nil, err
	}

	negotiated, err := negotiateCapabilities(open.Capabilities, r.opts.Capabilities)
	if err != nil {
		_ = r.sendReject(ctx, t, env, arcp.Code(err), err.Error())
		return nil, err
	}

	sess := &session{
		ID:           arcp.NewSessionID(),
		Principal:    principal,
		Capabilities: negotiated,
	}
	accept := arcp.Envelope{
		ID:            arcp.NewMessageID(),
		Timestamp:     time.Now().UTC(),
		SessionID:     sess.ID,
		CorrelationID: env.ID,
		Payload: &messages.SessionAccepted{
			SessionID:    sess.ID,
			Runtime:      r.opts.Identity,
			Capabilities: negotiated,
		},
	}
	if err := t.Send(ctx, accept); err != nil {
		return nil, fmt.Errorf("handshake: send session.accepted: %w", err)
	}
	r.persist(ctx, accept)
	r.persistOpen(ctx, env, sess.ID)
	return sess, nil
}

// runLoop processes envelopes after the handshake completes. v0.1
// understands ping/session.close natively and NACKs unknown core
// types per RFC §21.3.
func (r *Runtime) runLoop(ctx context.Context, t transport.Transport, sess *session) error {
	for {
		env, err := t.Recv(ctx)
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		}
		// Stamp session id on inbound envelope for log entries.
		if env.SessionID == "" {
			env.SessionID = sess.ID
		}
		r.persist(ctx, env)
		switch p := env.Payload.(type) {
		case *messages.Ping:
			pong := arcp.Envelope{
				ID:            arcp.NewMessageID(),
				Timestamp:     time.Now().UTC(),
				SessionID:     sess.ID,
				CorrelationID: env.ID,
				Payload:       &messages.Pong{Note: p.Note},
			}
			if err := t.Send(ctx, pong); err != nil {
				return err
			}
			r.persist(ctx, pong)
		case *messages.SessionClose:
			r.opts.Logger.Info("session closed by peer", "session_id", sess.ID, "reason", p.Reason)
			return nil
		default:
			// For anything not yet implemented, NACK with UNIMPLEMENTED
			// per RFC §21.3.
			nack := arcp.Envelope{
				ID:            arcp.NewMessageID(),
				Timestamp:     time.Now().UTC(),
				SessionID:     sess.ID,
				CorrelationID: env.ID,
				Payload: &messages.Nack{
					Code:    arcp.CodeUnimplemented,
					Message: fmt.Sprintf("type %q not yet implemented in v0.1", env.Type()),
				},
			}
			if err := t.Send(ctx, nack); err != nil {
				return err
			}
			r.persist(ctx, nack)
		}
	}
}

func (r *Runtime) sendReject(ctx context.Context, t transport.Transport, source arcp.Envelope, code arcp.ErrorCode, msg string) error {
	env := arcp.Envelope{
		ID:            arcp.NewMessageID(),
		Timestamp:     time.Now().UTC(),
		CorrelationID: source.ID,
		Payload:       &messages.SessionRejected{Code: code, Message: msg},
	}
	return t.Send(ctx, env)
}

func (r *Runtime) sendUnauthenticated(ctx context.Context, t transport.Transport, source arcp.Envelope, code arcp.ErrorCode, msg string) error {
	env := arcp.Envelope{
		ID:            arcp.NewMessageID(),
		Timestamp:     time.Now().UTC(),
		CorrelationID: source.ID,
		Payload:       &messages.SessionUnauthenticated{Code: code, Message: msg},
	}
	return t.Send(ctx, env)
}

// persist stores the envelope to the event log if one is configured.
// Errors are logged but not fatal.
func (r *Runtime) persist(ctx context.Context, env arcp.Envelope) {
	if r.opts.EventLog == nil {
		return
	}
	if env.SessionID == "" {
		return
	}
	if err := r.opts.EventLog.Append(ctx, env); err != nil {
		// Duplicate id is expected on retransmit and not an error.
		if errors.Is(err, arcp.ErrAlreadyExists) {
			return
		}
		r.opts.Logger.Warn("event log append failed", "error", err, "type", env.Type())
	}
}

// persistOpen records the original session.open envelope under the
// newly assigned session id (the inbound open did not yet have it).
func (r *Runtime) persistOpen(ctx context.Context, source arcp.Envelope, sid arcp.SessionID) {
	if r.opts.EventLog == nil {
		return
	}
	source.SessionID = sid
	if err := r.opts.EventLog.Append(ctx, source); err != nil &&
		!errors.Is(err, arcp.ErrAlreadyExists) {
		r.opts.Logger.Warn("event log append failed", "error", err, "type", source.Type())
	}
}

// negotiateCapabilities computes the intersection of client and server
// capabilities. Boolean caps are AND-ed; numeric caps are min'd;
// extension lists are intersected.
//
// Required-but-unsupported features (i.e. the client offers a
// must-have extension the runtime does not advertise) are signalled by
// returning an error with code UNIMPLEMENTED, per RFC §7.
func negotiateCapabilities(client, server messages.Capabilities) (messages.Capabilities, error) {
	out := intersectBoolCaps(client, server)
	out.HeartbeatRecovery = server.HeartbeatRecovery
	out.HeartbeatIntervalSeconds = negotiateHeartbeatInterval(client.HeartbeatIntervalSeconds, server.HeartbeatIntervalSeconds)
	out.BinaryEncoding = intersect(client.BinaryEncoding, server.BinaryEncoding)
	out.Extensions = intersect(client.Extensions, server.Extensions)

	// Required extensions: any client extension not in server's
	// advertised list is rejected.
	for _, ce := range client.Extensions {
		if !contains(server.Extensions, ce) {
			return messages.Capabilities{}, arcp.NewError(arcp.CodeUnimplemented,
				fmt.Sprintf("runtime does not advertise required extension %q", ce))
		}
	}
	return out, nil
}

// intersectBoolCaps returns a Capabilities with each boolean field set
// to client.X && server.X. Lifting it out of negotiateCapabilities
// keeps the latter under the cyclomatic-complexity gate.
func intersectBoolCaps(client, server messages.Capabilities) messages.Capabilities {
	return messages.Capabilities{
		Streaming:     client.Streaming && server.Streaming,
		DurableJobs:   client.DurableJobs && server.DurableJobs,
		Checkpoints:   client.Checkpoints && server.Checkpoints,
		BinaryStreams: client.BinaryStreams && server.BinaryStreams,
		AgentHandoff:  client.AgentHandoff && server.AgentHandoff,
		HumanInput:    client.HumanInput && server.HumanInput,
		Artifacts:     client.Artifacts && server.Artifacts,
		Subscriptions: client.Subscriptions && server.Subscriptions,
		ScheduledJobs: client.ScheduledJobs && server.ScheduledJobs,
		Anonymous:     client.Anonymous && server.Anonymous,
		Interrupt:     client.Interrupt && server.Interrupt,
	}
}

// negotiateHeartbeatInterval returns the lower of client and server
// proposals, falling back to server if client did not propose.
func negotiateHeartbeatInterval(clientS, serverS int) int {
	if clientS <= 0 {
		return serverS
	}
	if serverS <= 0 || clientS < serverS {
		return clientS
	}
	return serverS
}

func intersect(a, b []string) []string {
	if len(a) == 0 || len(b) == 0 {
		return nil
	}
	bset := make(map[string]struct{}, len(b))
	for _, x := range b {
		bset[x] = struct{}{}
	}
	out := make([]string, 0, len(a))
	for _, x := range a {
		if _, ok := bset[x]; ok {
			out = append(out, x)
		}
	}
	return out
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
