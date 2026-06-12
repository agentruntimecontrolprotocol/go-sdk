package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
	"github.com/agentruntimecontrolprotocol/go-sdk/credentials"
	"github.com/agentruntimecontrolprotocol/go-sdk/internal/clock"
	"github.com/agentruntimecontrolprotocol/go-sdk/internal/eventlog"
	"github.com/agentruntimecontrolprotocol/go-sdk/internal/idstore"
	"github.com/agentruntimecontrolprotocol/go-sdk/messages"
	"github.com/agentruntimecontrolprotocol/go-sdk/transport"
)

// session wraps one connected client.
type session struct {
	srv       *Server
	id        string
	principal string
	transport transport.Transport
	features  []string
	clientCap messages.HelloCapabilities

	ctx    context.Context
	cancel context.CancelFunc

	outbox chan arcp.Envelope
	wg     sync.WaitGroup

	sendMu       sync.Mutex
	outboxClosed bool
	// sendWG tracks in-flight outbox enqueues so closeOutbox can wait
	// for them to finish before closing the channel, letting enqueue
	// perform its (possibly blocking) channel send without holding
	// sendMu the whole time (#83).
	sendWG sync.WaitGroup

	// hbMu guards heartbeatTimer, the outbound session.ping ticker.
	// Per §6.4 idleness is measured per-direction: the timer is reset
	// whenever the server actually writes an envelope to the client,
	// not when it receives one, so a chatty client cannot suppress the
	// server's own keepalives.
	hbMu           sync.Mutex
	heartbeatTimer clock.Timer

	// seq is the per-session-id event_seq allocator. It is shared
	// across this session struct and any successor created by resume
	// so events emitted by a job that survives a disconnect cannot
	// collide with events emitted by the resumed session.
	seq *seqAlloc

	highMu sync.Mutex
	high   uint64 // last_processed_seq from session.ack

	// backPressureArmed reports whether the next ack-lag breach should
	// emit a status event. It is true at session start, set to false
	// when an event fires, and re-armed when the lag drops back below
	// the threshold.
	backPressureArmed atomic.Bool

	heartbeatLost atomic.Bool
	// gracefulBye is set when the client (or server) sends session.bye.
	// It suppresses resume-state stashing on session exit.
	gracefulBye atomic.Bool
	// resumeToken is the resume_token sent in this session's welcome.
	// Used to seed the resumeEntry on non-graceful exit so the client
	// can present it back on a subsequent hello.Resume.
	resumeToken string

	// isResume reports whether this session was created by a resume
	// handshake; if so, run() replays buffered events with
	// EventSeq > replayFrom after writeLoop is up.
	isResume   bool
	replayFrom uint64
}

func (srv *Server) handshake(ctx context.Context, t transport.Transport) (*session, error) {
	env, err := t.Recv(ctx)
	if err != nil {
		return nil, fmt.Errorf("await hello: %w", err)
	}
	if env.Type != messages.TypeSessionHello {
		return nil, arcp.ErrInvalidRequest.WithMessage("first envelope must be session.hello, got " + env.Type)
	}
	var hello messages.SessionHello
	if err := env.DecodePayload(&hello); err != nil {
		return nil, err
	}
	var principal string
	if srv.opts.Verifier != nil {
		p, err := srv.opts.Verifier.Verify(ctx, hello.Auth.Token)
		if err != nil {
			_ = sendSessionError(ctx, t, "", arcp.CodeUnauthenticated, err.Error())
			return nil, err
		}
		principal = p
	} else {
		principal = hello.Client.Name
	}
	// Resume path: validate the resume block, reuse the prior
	// session_id and seq counter, rotate the resume_token, and replay
	// every event with seq > hello.Resume.LastEventSeq.
	if hello.Resume != nil {
		entry, rerr := srv.claimResume(*hello.Resume, principal)
		if rerr != nil {
			_ = sendSessionError(ctx, t, hello.Resume.SessionID, arcp.Code(rerr), rerr.Error())
			return nil, rerr
		}
		// §6.3: if the buffer no longer covers the requested
		// last_event_seq (a gap between it and the oldest retained
		// event), reject with RESUME_WINDOW_EXPIRED rather than
		// replaying a silently-truncated stream (#136).
		if hello.Resume.LastEventSeq > 0 {
			if oldest, ok := srv.log.Oldest(entry.sessionID); ok && oldest > hello.Resume.LastEventSeq+1 {
				_ = sendSessionError(ctx, t, entry.sessionID, arcp.CodeResumeWindowExpired, "resume buffer no longer covers last_event_seq")
				return nil, arcp.ErrResumeWindowExpired.WithMessage("resume buffer no longer covers last_event_seq")
			}
		}
		feats := arcp.IntersectFeatures(srv.features(), hello.Capabilities.Features)
		newToken := arcp.NewULID()
		wenv, err := srv.buildWelcome(entry.sessionID, newToken, feats)
		if err != nil {
			return nil, err
		}
		if err := t.Send(ctx, wenv); err != nil {
			return nil, fmt.Errorf("send welcome: %w", err)
		}
		sctx, cancel := context.WithCancel(ctx)
		alloc := srv.allocFor(entry.sessionID)
		alloc.setIfGreater(entry.seq)
		s := srv.newSession(entry.sessionID, principal, t, feats, hello.Capabilities, sctx, cancel, alloc, newToken)
		s.isResume = true
		s.replayFrom = hello.Resume.LastEventSeq
		// The buffered-event replay is deferred to run(): doing it here
		// (before writeLoop exists to drain the outbox) deadlocks once
		// more than the outbox capacity is buffered (#146).
		return s, nil
	}
	sessionID := arcp.NewSessionID()
	feats := arcp.IntersectFeatures(srv.features(), hello.Capabilities.Features)
	resumeToken := arcp.NewULID()
	wenv, err := srv.buildWelcome(sessionID, resumeToken, feats)
	if err != nil {
		return nil, err
	}
	if err := t.Send(ctx, wenv); err != nil {
		return nil, fmt.Errorf("send welcome: %w", err)
	}
	sctx, cancel := context.WithCancel(ctx)
	s := srv.newSession(sessionID, principal, t, feats, hello.Capabilities, sctx, cancel, srv.allocFor(sessionID), resumeToken)
	return s, nil
}

// buildWelcome constructs the session.welcome envelope shared by the
// fresh and resume handshake paths (#99).
func (srv *Server) buildWelcome(sessionID, token string, feats []string) (arcp.Envelope, error) {
	welcome := messages.SessionWelcome{
		Runtime:              messages.RuntimeInfo{Name: srv.opts.Name, Version: srv.opts.Version},
		ResumeToken:          token,
		ResumeWindowSec:      int(srv.opts.ResumeWindow / time.Second),
		HeartbeatIntervalSec: int(srv.opts.HeartbeatInterval / time.Second),
		Capabilities: messages.WelcomeCapabilities{
			Encodings: []string{"json"},
			Features:  feats,
			Agents:    srv.inventory(),
		},
	}
	wenv, err := arcp.NewEnvelope(messages.TypeSessionWelcome, &welcome)
	if err != nil {
		return arcp.Envelope{}, err
	}
	wenv.SessionID = sessionID
	return wenv, nil
}

// newSession constructs a session struct shared by both handshake paths
// (#99). The outbox capacity is configurable via Options.OutboxBuffer
// (#98).
func (srv *Server) newSession(id, principal string, t transport.Transport, feats []string, clientCap messages.HelloCapabilities, ctx context.Context, cancel context.CancelFunc, seq *seqAlloc, resumeToken string) *session {
	s := &session{
		srv:         srv,
		id:          id,
		principal:   principal,
		transport:   t,
		features:    feats,
		clientCap:   clientCap,
		ctx:         ctx,
		cancel:      cancel,
		outbox:      make(chan arcp.Envelope, srv.opts.OutboxBuffer),
		seq:         seq,
		resumeToken: resumeToken,
	}
	s.backPressureArmed.Store(true)
	return s
}

func sendSessionError(ctx context.Context, t transport.Transport, sessionID string, code arcp.ErrorCode, msg string) error {
	body := messages.SessionError{Code: code, Message: msg, Retryable: code == arcp.CodeInternalError}
	env, err := arcp.NewEnvelope(messages.TypeSessionError, &body)
	if err != nil {
		return err
	}
	env.SessionID = sessionID
	return t.Send(ctx, env)
}

func (s *session) hasFeature(name string) bool {
	return arcp.HasFeature(s.features, name)
}

// nextSeq allocates the next session-scoped event_seq from the
// session-id-shared allocator so the counter survives reconnects.
func (s *session) nextSeq() uint64 {
	return s.seq.next()
}

// currentSeq returns the most recently allocated seq without consuming
// one.
func (s *session) currentSeq() uint64 {
	return s.seq.current()
}

// send enqueues env on the outbox. Sequenced events are also persisted
// to the event log immediately so jobs that outlive the transport
// (the resume contract) still produce replayable history. Best-effort
// transport delivery only happens when the outbox is live.
func (s *session) send(env arcp.Envelope) {
	env.SessionID = s.id
	s.persistOutbound(env)
	// Resolve the live session for this session id so an event emitted
	// by a job whose original session was replaced by a resume reaches
	// the reconnected transport instead of the original closed outbox
	// (#145). Persistence above is keyed by session id and is unchanged.
	cur := s.srv.currentSession(s.id)
	if cur == nil {
		cur = s
	}
	cur.enqueue(env)
}

// sendEphemeral delivers env to the live session for this id WITHOUT
// persisting it to the event log. Used for credential rotation, whose
// payload carries a secret that must never enter the replayable log.
func (s *session) sendEphemeral(env arcp.Envelope) {
	env.SessionID = s.id
	cur := s.srv.currentSession(s.id)
	if cur == nil {
		cur = s
	}
	cur.enqueue(env)
}

// enqueue places env on this session's outbox (best-effort while the
// outbox is live) and evaluates back-pressure. Delivery is skipped once
// the outbox is closed; the event is still durable via the log. sendMu
// is held only to check the closed flag and register with sendWG; the
// blocking channel send happens without the lock so a slow transport
// does not serialize all event production (#83). closeOutbox always
// cancels s.ctx first, so a parked send unblocks via the ctx.Done case
// before the channel is closed.
func (s *session) enqueue(env arcp.Envelope) {
	s.sendMu.Lock()
	if s.outboxClosed {
		s.sendMu.Unlock()
		return
	}
	s.sendWG.Add(1)
	s.sendMu.Unlock()
	defer s.sendWG.Done()
	select {
	case s.outbox <- env:
	case <-s.ctx.Done():
	}
	s.maybeEmitBackPressure(env.JobID, env.EventSeq)
}

// replayBuffered re-delivers persisted events with EventSeq > fromSeq to
// this (resumed) session's outbox. It runs from run() after writeLoop is
// started so the outbox is drained concurrently (avoiding the >128-event
// deadlock, #146). Replayed envelopes already carry their EventSeq and
// are already persisted, so they are enqueued directly rather than via
// send (no double-persist, #76).
func (s *session) replayBuffered(fromSeq uint64) {
	entries, err := s.srv.log.Since(s.id, fromSeq)
	if err != nil {
		// Surface the error rather than silently serving stale/no data
		// (#78); the client can re-resume.
		s.srv.opts.Logger.Debug("resume replay: log.Since failed", "session", s.id, "err", err)
		return
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].EventSeq < entries[j].EventSeq
	})
	for _, e := range entries {
		env := e.Envelope
		env.SessionID = s.id
		s.enqueue(env)
	}
}

// persistOutbound appends env to the per-session event log when it
// carries a session-scoped event sequence and is not a credential
// rotation (which is not replayable for security reasons).
func (s *session) persistOutbound(env arcp.Envelope) {
	if env.EventSeq == 0 {
		return
	}
	if isCredentialRotation(env) {
		return
	}
	_ = s.srv.log.Append(eventlog.Entry{
		SessionID: s.id,
		EventSeq:  env.EventSeq,
		JobID:     env.JobID,
		StoredAt:  s.srv.opts.Clock.Now(),
		Envelope:  env,
	})
}

func (s *session) closeOutbox() {
	s.sendMu.Lock()
	if s.outboxClosed {
		s.sendMu.Unlock()
		return
	}
	s.outboxClosed = true
	s.sendMu.Unlock()
	// Wait for in-flight enqueues to drain (they unblock via s.ctx.Done,
	// which the caller cancels before closeOutbox) before closing the
	// channel so no send races a close (#83).
	s.sendWG.Wait()
	close(s.outbox)
}

// writeLoop drains outbox; one writer per session. The event log is
// populated by send/persistOutbound at enqueue time, not here, so
// events emitted while the transport is down still produce log
// entries.
func (s *session) writeLoop() {
	defer s.wg.Done()
	for {
		select {
		case env, ok := <-s.outbox:
			if !ok {
				return
			}
			if err := s.transport.Send(s.ctx, env); err != nil {
				if !errors.Is(err, context.Canceled) {
					s.srv.opts.Logger.Debug("send envelope failed", "err", err, "type", env.Type)
				}
				return
			}
			// A message just flowed in the server→client direction, so
			// reset the outbound heartbeat timer (§6.4 per-direction
			// idleness).
			s.hbMu.Lock()
			if s.heartbeatTimer != nil {
				s.heartbeatTimer.Reset(s.srv.opts.HeartbeatInterval)
			}
			s.hbMu.Unlock()
		case <-s.ctx.Done():
			return
		}
	}
}

// maybeEmitBackPressure emits a single back_pressure status event when
// the gap between the outbound event sequence and the highest ack
// crosses Options.AckLagThreshold. The breach is coalesced: subsequent
// events do not re-fire until handleAck observes the gap drop back
// below the threshold. The emitted event itself bumps the seq counter
// but re-entering this function from its own send is harmless because
// backPressureArmed is already false until the next ack re-arms.
func (s *session) maybeEmitBackPressure(jobID string, seq uint64) {
	threshold := s.srv.opts.AckLagThreshold
	if threshold == 0 || !s.hasFeature("ack") {
		return
	}
	s.highMu.Lock()
	high := s.high
	s.highMu.Unlock()
	if seq <= high || seq-high < threshold {
		return
	}
	if !s.backPressureArmed.CompareAndSwap(true, false) {
		return
	}
	body := messages.StatusBody{
		Phase:   "back_pressure",
		Message: "unacked event lag exceeded threshold",
		Details: map[string]any{
			"threshold":   threshold,
			"current_seq": seq,
			"last_ack":    high,
		},
	}
	raw, err := json.Marshal(body)
	if err != nil {
		s.backPressureArmed.Store(true)
		return
	}
	ev := messages.JobEvent{
		Kind: messages.KindStatus,
		TS:   s.srv.opts.Clock.Now(),
		Body: raw,
	}
	env, err := arcp.NewEnvelope(messages.TypeJobEvent, &ev)
	if err != nil {
		s.backPressureArmed.Store(true)
		return
	}
	env.JobID = jobID
	env.SessionID = s.id
	env.EventSeq = s.nextSeq()
	// Persist then enqueue directly with a NON-blocking send rather than
	// recursing through send/enqueue (which would re-enter this function
	// and could block on a saturated outbox). Dropping the back_pressure
	// event under saturation is acceptable; it re-arms on the next ack
	// that drops the lag below the threshold (#75).
	s.persistOutbound(env)
	s.sendMu.Lock()
	if !s.outboxClosed {
		select {
		case s.outbox <- env:
		default:
			// Outbox saturated; re-arm so the breach can fire later.
			s.backPressureArmed.Store(true)
		}
	}
	s.sendMu.Unlock()
}

func isCredentialRotation(env arcp.Envelope) bool {
	if env.Type != messages.TypeJobEvent {
		return false
	}
	var ev messages.JobEvent
	if err := env.DecodePayload(&ev); err != nil || ev.Kind != messages.KindStatus {
		return false
	}
	var body messages.StatusBody
	if err := json.Unmarshal(ev.Body, &body); err != nil {
		return false
	}
	return body.Phase == messages.PhaseCredentialRotated
}

// run drives the read loop. Returns when the session ends.
func (s *session) run(ctx context.Context) error {
	// Become the live session for this id before starting the writer so
	// that events from surviving jobs route to this transport (#145).
	s.srv.setCurrentSession(s)
	s.wg.Add(1)
	go s.writeLoop()

	// Replay buffered events now that writeLoop is draining the outbox
	// (#146); this also delivers anything a surviving job emitted while
	// the transport was down.
	if s.isResume {
		s.replayBuffered(s.replayFrom)
	}

	defer func() {
		s.srv.clearCurrentSession(s)
		s.cancel()
		s.closeOutbox()
		s.wg.Wait()
		_ = s.transport.Close()
	}()

	// Outbound heartbeat ticker.
	var heartbeatTimer clock.Timer
	if s.hasFeature("heartbeat") && s.srv.opts.HeartbeatInterval > 0 {
		heartbeatTimer = s.srv.opts.Clock.AfterFunc(s.srv.opts.HeartbeatInterval, s.sendPing)
		s.hbMu.Lock()
		s.heartbeatTimer = heartbeatTimer
		s.hbMu.Unlock()
		defer heartbeatTimer.Stop()
	}
	// Inbound heartbeat watchdog.
	var watchdog clock.Timer
	if s.hasFeature("heartbeat") && s.srv.opts.HeartbeatInterval > 0 {
		watchdog = s.srv.opts.Clock.AfterFunc(2*s.srv.opts.HeartbeatInterval, func() {
			s.heartbeatLost.Store(true)
			// Tell the client why the connection is dropping (#87).
			if env, err := arcp.NewEnvelope(messages.TypeSessionError, &messages.SessionError{
				Code:      arcp.CodeHeartbeatLost,
				Message:   "heartbeat lost: no inbound traffic within two intervals",
				Retryable: true,
			}); err == nil {
				env.SessionID = s.id
				_ = s.transport.Send(s.ctx, env)
			}
			s.cancel()
		})
		defer watchdog.Stop()
	}

	for {
		env, err := s.transport.Recv(s.ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil
			}
			return err
		}
		if watchdog != nil {
			watchdog.Reset(2 * s.srv.opts.HeartbeatInterval)
		}
		// Note: the outbound heartbeat timer is intentionally NOT reset
		// here. Per §6.4, heartbeat idleness is measured per-direction;
		// the outbound timer is reset only when the server writes (see
		// writeLoop), so inbound chatter cannot starve server pings.
		if err := s.dispatch(s.ctx, env); err != nil {
			s.srv.opts.Logger.Debug("dispatch failed", "type", env.Type, "err", err)
		}
	}
}

func (s *session) sendPing() {
	if !s.hasFeature("heartbeat") {
		return
	}
	ping := messages.SessionPing{
		Nonce:  arcp.NewPingNonce(),
		SentAt: s.srv.opts.Clock.Now(),
	}
	env, err := arcp.NewEnvelope(messages.TypeSessionPing, &ping)
	if err != nil {
		return
	}
	s.send(env)
}

func (s *session) dispatch(ctx context.Context, env arcp.Envelope) error {
	switch env.Type {
	case messages.TypeSessionPing:
		return s.handlePing(env)
	case messages.TypeSessionPong:
		return nil
	case messages.TypeSessionAck:
		return s.handleAck(env)
	case messages.TypeSessionListJobs:
		return s.handleListJobs(ctx, env)
	case messages.TypeSessionClose:
		var body messages.SessionClose
		_ = env.DecodePayload(&body)
		s.gracefulBye.Store(true)
		// Acknowledge with session.closed (§6.7) before tearing down.
		// Sent synchronously here (the shipped transports guard writes)
		// so the ack is delivered before cancel stops the writer.
		closed, err := arcp.NewEnvelope(messages.TypeSessionClosed, &messages.SessionClosed{Reason: body.Reason})
		if err == nil {
			closed.SessionID = s.id
			_ = s.transport.Send(s.ctx, closed)
		}
		s.cancel()
		return nil
	case messages.TypeJobSubmit:
		return s.handleJobSubmit(ctx, env)
	case messages.TypeJobCancel:
		return s.handleJobCancel(env)
	case messages.TypeJobSubscribe:
		return s.handleJobSubscribe(ctx, env)
	case messages.TypeJobUnsubscribe:
		return s.handleJobUnsubscribe(env)
	default:
		return s.sendErrorFor(env, arcp.CodeInvalidRequest, "unknown envelope type "+env.Type)
	}
}

// sendErrorFor emits a per-request session.error that echoes the
// offending request's envelope id and job_id so the client can fail
// only the originating call instead of tearing down the whole session.
func (s *session) sendErrorFor(reqEnv arcp.Envelope, code arcp.ErrorCode, msg string) error {
	body := messages.SessionError{
		Code:      code,
		Message:   msg,
		Retryable: code == arcp.CodeInternalError,
		RequestID: reqEnv.ID,
		JobID:     reqEnv.JobID,
	}
	out, err := arcp.NewEnvelope(messages.TypeSessionError, &body)
	if err != nil {
		return err
	}
	out.JobID = reqEnv.JobID
	s.send(out)
	return nil
}

func (s *session) handlePing(env arcp.Envelope) error {
	var ping messages.SessionPing
	if err := env.DecodePayload(&ping); err != nil {
		return err
	}
	pong := messages.SessionPong{PingNonce: ping.Nonce, ReceivedAt: s.srv.opts.Clock.Now()}
	out, err := arcp.NewEnvelope(messages.TypeSessionPong, &pong)
	if err != nil {
		return err
	}
	s.send(out)
	return nil
}

func (s *session) handleAck(env arcp.Envelope) error {
	if !s.hasFeature("ack") {
		return s.sendErrorFor(env, arcp.CodeInvalidRequest, "ack feature not negotiated")
	}
	var ack messages.SessionAck
	if err := env.DecodePayload(&ack); err != nil {
		return err
	}
	s.highMu.Lock()
	if ack.LastProcessedSeq > s.high {
		s.high = ack.LastProcessedSeq
	}
	high := s.high
	s.highMu.Unlock()
	_ = s.srv.log.Trim(s.id, ack.LastProcessedSeq)
	// Re-arm back_pressure once the client has caught back up below
	// the threshold; the next breach will fire a fresh status event.
	if threshold := s.srv.opts.AckLagThreshold; threshold > 0 {
		// Guard the unsigned subtraction: an ack whose LastProcessedSeq
		// exceeds currentSeq (e.g. after resume replay) would otherwise
		// underflow to a huge gap and mis-arm back_pressure (#70).
		if cur := s.currentSeq(); cur <= high || cur-high < threshold {
			s.backPressureArmed.Store(true)
		}
	}
	return nil
}

func (s *session) handleListJobs(ctx context.Context, env arcp.Envelope) error {
	if !s.hasFeature("list_jobs") {
		return s.sendErrorFor(env, arcp.CodeInvalidRequest, "list_jobs feature not negotiated")
	}
	var req messages.SessionListJobs
	if err := env.DecodePayload(&req); err != nil {
		return err
	}
	jobs, next, err := s.srv.listJobs(s.principal, req.Filter, req.Limit, req.Cursor)
	if err != nil {
		return s.sendErrorFor(env, arcp.CodeInternalError, err.Error())
	}
	resp := messages.SessionJobs{
		RequestID:  env.ID,
		Jobs:       jobs,
		NextCursor: next,
	}
	out, err := arcp.NewEnvelope(messages.TypeSessionJobs, &resp)
	if err != nil {
		return err
	}
	s.send(out)
	return nil
}

func (s *session) handleJobSubmit(ctx context.Context, env arcp.Envelope) error {
	var req messages.JobSubmit
	if err := env.DecodePayload(&req); err != nil {
		return s.sendErrorFor(env, arcp.CodeInvalidRequest, err.Error())
	}
	ref, err := messages.ParseAgentRef(req.Agent)
	if err != nil {
		return s.sendErrorFor(env, arcp.CodeInvalidRequest, err.Error())
	}
	fn, canonical, err := s.srv.resolveAgent(ref)
	if err != nil {
		return s.sendErrorFor(env, arcp.Code(err), err.Error())
	}
	if req.LeaseConstraints != nil && req.LeaseConstraints.ExpiresAt != nil {
		// §9.5: expires_at MUST be UTC (Z / +00:00). Reject any other
		// zone offset as INVALID_REQUEST.
		if _, off := req.LeaseConstraints.ExpiresAt.Zone(); off != 0 {
			return s.sendErrorFor(env, arcp.CodeInvalidRequest, "lease_constraints.expires_at must be UTC (Z suffix)")
		}
		if !req.LeaseConstraints.ExpiresAt.After(s.srv.opts.Clock.Now()) {
			return s.sendErrorFor(env, arcp.CodeInvalidRequest, "lease_constraints.expires_at must be in the future")
		}
	}
	// Validate cost.budget grammar up front (§9.6/§12). A malformed
	// budget pattern must reject the submission with INVALID_REQUEST
	// rather than silently running the job with no budget bound.
	for _, pat := range req.LeaseRequest[arcp.CapCostBudget] {
		if _, perr := arcp.ParseBudgetAmount(pat); perr != nil {
			return s.sendErrorFor(env, arcp.CodeInvalidRequest, "lease_request cost.budget: "+perr.Error())
		}
	}
	job := newJob(s, canonical, req, fn, env.TraceID)
	if !s.srv.registerJob(job) {
		job.discard()
		return s.sendErrorFor(env, arcp.CodeInternalError, "job id collision")
	}
	paramsHash := submitParamsHash(req)
	if req.IdempotencyKey != "" {
		entry, fresh, err := s.srv.idStore.PutIfAbsent(ctx, idstore.Entry{
			Principal:  s.principal,
			Key:        req.IdempotencyKey,
			JobID:      job.id,
			CreatedAt:  s.srv.opts.Clock.Now(),
			ParamsHash: paramsHash,
		})
		if err != nil {
			s.srv.unregisterJob(job.id)
			job.discard()
			return s.sendErrorFor(env, arcp.Code(err), "idempotency store error: "+err.Error())
		}
		if !fresh {
			s.srv.unregisterJob(job.id)
			job.discard()
			// §7.2: identical retry replays the original job.accepted;
			// a conflicting reuse returns DUPLICATE_KEY.
			if entry.ParamsHash != paramsHash {
				return s.sendErrorFor(env, arcp.CodeDuplicateKey, "idempotency key reused with different parameters")
			}
			if len(entry.Accepted) == 0 {
				return s.sendErrorFor(env, arcp.CodeInternalError, "idempotent submit still in progress")
			}
			return s.replayAccepted(env, entry)
		}
	}
	accept := messages.JobAccepted{
		JobID:            job.id,
		Lease:            job.lease.Lease(),
		LeaseConstraints: leaseConstraintsFromState(job.lease.ExpiresAt()),
		Budget:           job.lease.Initial(),
		AcceptedAt:       s.srv.opts.Clock.Now(),
		TraceID:          job.traceID,
		Agent:            canonical,
	}
	if s.hasFeature("provisioned_credentials") && s.srv.opts.Provisioner != nil {
		creds, err := s.srv.opts.Provisioner.Issue(ctx, credentials.IssueRequest{
			JobID:     job.id,
			Principal: s.principal,
			Agent:     canonical,
			Lease:     job.lease.Lease(),
			Budget:    job.lease.Initial(),
			ExpiresAt: job.lease.ExpiresAt(),
		})
		if err != nil {
			s.srv.unregisterJob(job.id)
			job.discard()
			return s.sendErrorFor(env, arcp.Code(err), "credential issue failed: "+err.Error())
		}
		job.attachCredentials(creds)
		accept.Credentials = creds
	}
	aenv, err := arcp.NewEnvelope(messages.TypeJobAccepted, &accept)
	if err != nil {
		s.srv.unregisterJob(job.id)
		job.discard()
		return err
	}
	aenv.JobID = job.id
	aenv.TraceID = job.traceID
	s.send(aenv)
	// Cache the original job.accepted so an identical retry under the
	// same idempotency_key can be replayed (§7.2).
	if req.IdempotencyKey != "" {
		if raw, mErr := json.Marshal(&accept); mErr == nil {
			_ = s.srv.idStore.SetAccepted(ctx, s.principal, req.IdempotencyKey, raw)
		}
	}
	go job.run()
	return nil
}

// replayAccepted re-emits the cached job.accepted for an idempotent
// retry that matches the original parameters.
func (s *session) replayAccepted(reqEnv arcp.Envelope, entry idstore.Entry) error {
	var acc messages.JobAccepted
	if err := json.Unmarshal(entry.Accepted, &acc); err != nil {
		return s.sendErrorFor(reqEnv, arcp.CodeInternalError, "cached job.accepted decode: "+err.Error())
	}
	out, err := arcp.NewEnvelope(messages.TypeJobAccepted, &acc)
	if err != nil {
		return err
	}
	out.JobID = acc.JobID
	out.TraceID = acc.TraceID
	s.send(out)
	return nil
}

// submitParamsHash returns a canonical hash of the submit parameters
// that define job identity for idempotency (§7.2), excluding the
// idempotency key and trace id.
func submitParamsHash(req messages.JobSubmit) string {
	canon := struct {
		Agent       string                     `json:"agent"`
		Input       json.RawMessage            `json:"input,omitempty"`
		Lease       arcp.Lease                 `json:"lease_request,omitempty"`
		Constraints *messages.LeaseConstraints `json:"lease_constraints,omitempty"`
		MaxRuntime  int                        `json:"max_runtime_sec,omitempty"`
	}{
		Agent:       req.Agent,
		Input:       req.Input,
		Lease:       req.LeaseRequest,
		Constraints: req.LeaseConstraints,
		MaxRuntime:  req.MaxRuntimeSec,
	}
	b, _ := json.Marshal(canon)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func leaseConstraintsFromState(t *time.Time) *messages.LeaseConstraints {
	if t == nil {
		return nil
	}
	return &messages.LeaseConstraints{ExpiresAt: t}
}

func (s *session) handleJobCancel(env arcp.Envelope) error {
	var req messages.JobCancel
	if err := env.DecodePayload(&req); err != nil {
		// A malformed cancel must be rejected, not coerced into a
		// zero-reason cancel (#74).
		return s.sendErrorFor(env, arcp.CodeInvalidRequest, err.Error())
	}
	job := s.srv.lookupJob(env.JobID)
	if job == nil {
		return s.sendErrorFor(env, arcp.CodeJobNotFound, "job "+env.JobID+" not found")
	}
	// Authorize by stable session id + principal rather than the live
	// *session pointer, so a resumed connection (new session struct,
	// same id) retains cancel authority (§7.4/§7.7, #137).
	if job.session.id != s.id || job.principal != s.principal {
		return s.sendErrorFor(env, arcp.CodePermissionDenied, "only the submitting session can cancel")
	}
	// Acknowledge with job.cancelled (§7.4) before the agent's unwind
	// path emits the terminal job.error{code:CANCELLED}.
	ack, err := arcp.NewEnvelope(messages.TypeJobCancelled, &messages.JobCancelled{JobID: job.id, Reason: req.Reason})
	if err == nil {
		ack.JobID = job.id
		ack.TraceID = job.traceID
		ack.EventSeq = s.nextSeq()
		s.send(ack)
	}
	job.cancelWithReason(req.Reason)
	return nil
}

func (s *session) handleJobSubscribe(ctx context.Context, env arcp.Envelope) error {
	if !s.hasFeature("subscribe") {
		return s.sendErrorFor(env, arcp.CodeInvalidRequest, "subscribe feature not negotiated")
	}
	var req messages.JobSubscribe
	if err := env.DecodePayload(&req); err != nil {
		return err
	}
	job := s.srv.lookupJob(req.JobID)
	if job == nil {
		s.srv.opts.Logger.Warn("job.subscribe denied",
			"subscriber_principal", s.principal, "job_id", req.JobID,
			"target_principal", "", "decision", "deny", "reason", "job_not_found")
		return s.sendErrorFor(env, arcp.CodeJobNotFound, "job "+req.JobID+" not found")
	}
	if job.principal != s.principal {
		s.srv.opts.Logger.Warn("job.subscribe denied",
			"subscriber_principal", s.principal, "job_id", req.JobID,
			"target_principal", job.principal, "decision", "deny", "reason", "principal_mismatch")
		return s.sendErrorFor(env, arcp.CodePermissionDenied, "subscription denied by deployment policy")
	}
	s.srv.opts.Logger.Info("job.subscribe granted",
		"subscriber_principal", s.principal, "job_id", req.JobID,
		"target_principal", job.principal, "decision", "grant")
	sub := newSubscription(s, req.JobID)
	s.srv.addSubscriber(req.JobID, sub)
	subscribed := messages.JobSubscribed{
		JobID:          job.id,
		CurrentStatus:  job.status(),
		Agent:          job.agent,
		Lease:          job.lease.Lease(),
		TraceID:        job.traceID,
		SubscribedFrom: s.currentSeq(),
		Replayed:       req.History,
	}
	out, err := arcp.NewEnvelope(messages.TypeJobSubscribed, &subscribed)
	if err != nil {
		return err
	}
	out.JobID = job.id
	s.send(out)
	if req.History {
		// Replay buffered events under the subscriber's seq space.
		// Enqueue directly (not via send) so the replay does not
		// re-persist duplicate log entries differing only by seq (#77).
		entries, _ := s.srv.log.SinceJob(job.id, req.FromEventSeq)
		for _, e := range entries {
			replay := e.Envelope
			replay.SessionID = s.id
			replay.EventSeq = s.nextSeq()
			s.enqueue(replay)
		}
	}
	return nil
}

func (s *session) handleJobUnsubscribe(env arcp.Envelope) error {
	var req messages.JobUnsubscribe
	if err := env.DecodePayload(&req); err != nil {
		return err
	}
	// Find any subscription owned by this session for jobID.
	s.srv.mu.RLock()
	subs := append([]*subscription(nil), s.srv.subs[req.JobID]...)
	s.srv.mu.RUnlock()
	for _, sub := range subs {
		if sub.session == s {
			s.srv.removeSubscriber(req.JobID, sub)
			sub.close()
			break
		}
	}
	return nil
}
