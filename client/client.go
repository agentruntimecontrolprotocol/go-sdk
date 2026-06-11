package client

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
	"github.com/agentruntimecontrolprotocol/go-sdk/messages"
	"github.com/agentruntimecontrolprotocol/go-sdk/transport"
)

// listResult carries either a session.jobs response or a correlated
// error to a pending ListJobs waiter.
type listResult struct {
	jobs *messages.SessionJobs
	err  error
}

// Client is the client-side view of one ARCP session. Construct with
// Connect.
type Client struct {
	opts      Options
	transport transport.Transport
	sessionID string
	welcome   *messages.SessionWelcome
	features  []string

	ctx    context.Context
	cancel context.CancelFunc

	// submitMu serializes the submit send + job.accepted wait so the
	// FIFO order of c.pending matches the order the runtime allocates
	// job ids. Until the wire payload carries a request id, this is
	// the only correlation guarantee available.
	submitMu sync.Mutex

	mu          sync.RWMutex
	handles     map[string]*JobHandle
	pending     []*JobHandle
	pendingByID map[string]*JobHandle
	subscribers map[string]*Subscription
	listReqs    map[string]chan listResult

	wg sync.WaitGroup

	highSeq atomic.Uint64
	lastAck atomic.Uint64

	// ackTickerStop signals the auto-ack interval flusher to exit on
	// Client.Close. nil when the flusher is not running (auto-ack
	// disabled or "ack" feature not negotiated).
	ackTickerStop chan struct{}
}

// Connect performs a session.hello / session.welcome handshake on t
// and returns a connected Client.
func Connect(ctx context.Context, t transport.Transport, opts Options) (*Client, error) {
	o := opts.withDefaults()
	hello := messages.SessionHello{
		Client: messages.ClientInfo{Name: o.ClientName, Version: o.ClientVersion},
		Auth:   messages.AuthInfo{Scheme: "bearer", Token: o.Token},
		Capabilities: messages.HelloCapabilities{
			Encodings: []string{"json"},
			Features:  o.Features,
		},
		Resume: o.Resume,
	}
	env, err := arcp.NewEnvelope(messages.TypeSessionHello, &hello)
	if err != nil {
		return nil, err
	}
	if err := t.Send(ctx, env); err != nil {
		return nil, fmt.Errorf("send hello: %w", err)
	}
	resp, err := t.Recv(ctx)
	if err != nil {
		return nil, fmt.Errorf("await welcome: %w", err)
	}
	switch resp.Type {
	case messages.TypeSessionWelcome:
	case messages.TypeSessionError:
		var serr messages.SessionError
		_ = resp.DecodePayload(&serr)
		return nil, &arcp.Error{Code: serr.Code, Message: serr.Message, Retryable: serr.Retryable, Details: serr.Details}
	default:
		return nil, arcp.ErrInvalidRequest.WithMessage("expected session.welcome, got " + resp.Type)
	}
	var welcome messages.SessionWelcome
	if err := resp.DecodePayload(&welcome); err != nil {
		return nil, err
	}
	cctx, cancel := context.WithCancel(ctx)
	c := &Client{
		opts:        o,
		transport:   t,
		sessionID:   resp.SessionID,
		welcome:     &welcome,
		features:    arcp.IntersectFeatures(o.Features, welcome.Capabilities.Features),
		ctx:         cctx,
		cancel:      cancel,
		handles:     map[string]*JobHandle{},
		pendingByID: map[string]*JobHandle{},
		subscribers: map[string]*Subscription{},
		listReqs:    map[string]chan listResult{},
	}
	c.wg.Add(1)
	go c.readLoop()
	c.startAutoAckTicker()
	return c, nil
}

// startAutoAckTicker emits a session.ack at most once per
// AutoAckInterval whenever the highest observed seq has advanced past
// the last acked value, so streams that deliver fewer than
// AutoAckWindow events still drain their ack debt. No-op when auto-ack
// is disabled or the "ack" feature is not negotiated.
func (c *Client) startAutoAckTicker() {
	if c.opts.AutoAckWindow == 0 {
		return
	}
	if !c.HasFeature("ack") {
		return
	}
	interval := c.opts.AutoAckInterval
	if interval <= 0 {
		return
	}
	stop := make(chan struct{})
	c.ackTickerStop = stop
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				current := c.highSeq.Load()
				last := c.lastAck.Load()
				if current > last && c.lastAck.CompareAndSwap(last, current) {
					c.sendAck(current)
				}
			case <-stop:
				return
			case <-c.ctx.Done():
				return
			}
		}
	}()
}

// SessionID returns the negotiated session identifier.
func (c *Client) SessionID() string { return c.sessionID }

// Welcome returns the welcome payload received from the runtime.
func (c *Client) Welcome() *messages.SessionWelcome { return c.welcome }

// Features returns the effective negotiated feature set.
func (c *Client) Features() []string { return c.features }

// HasFeature reports whether name was negotiated.
func (c *Client) HasFeature(name string) bool { return arcp.HasFeature(c.features, name) }

// HighestSeq returns the largest event_seq the client has seen on
// this session, suitable as the LastEventSeq value when constructing
// a messages.ResumeRequest for a subsequent reconnect.
func (c *Client) HighestSeq() uint64 { return c.highSeq.Load() }

// Close terminates the session.
func (c *Client) Close(ctx context.Context) error {
	env, _ := arcp.NewEnvelope(messages.TypeSessionClose, &messages.SessionClose{Reason: "client close"})
	env.SessionID = c.sessionID
	_ = c.transport.Send(ctx, env)
	c.cancel()
	if c.ackTickerStop != nil {
		close(c.ackTickerStop)
		c.ackTickerStop = nil
	}
	err := c.transport.Close()
	c.wg.Wait()
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, h := range c.handles {
		h.fail(arcp.ErrInternalError.WithMessage("client closed"))
	}
	for _, s := range c.subscribers {
		s.close(arcp.ErrInternalError.WithMessage("client closed"))
	}
	return err
}

// readLoop consumes envelopes and dispatches them to handles /
// subscribers / list waiters.
func (c *Client) readLoop() {
	defer c.wg.Done()
	for {
		env, err := c.transport.Recv(c.ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return
			}
			c.failAll(err)
			return
		}
		if env.EventSeq > 0 {
			c.highSeq.Store(env.EventSeq)
		}
		c.dispatch(env)
	}
}

func (c *Client) failAll(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, h := range c.handles {
		h.fail(err)
	}
	for _, h := range c.pending {
		h.fail(err)
	}
	c.pending = nil
	c.pendingByID = map[string]*JobHandle{}
	for _, s := range c.subscribers {
		s.close(err)
	}
	for id, ch := range c.listReqs {
		select {
		case ch <- listResult{err: err}:
		default:
		}
		delete(c.listReqs, id)
	}
}

// routeError delivers a per-request session.error to the single
// originating waiter — a pending submit, a pending/active subscription,
// a ListJobs call, or an already-accepted job handle — correlated by
// the echoed request id and job id. It returns true when a waiter was
// found, so the caller can decide whether the error is session-fatal.
func (c *Client) routeError(requestID, jobID string, e error) bool {
	c.mu.Lock()
	if requestID != "" {
		if h, ok := c.pendingByID[requestID]; ok {
			delete(c.pendingByID, requestID)
			for i, ph := range c.pending {
				if ph == h {
					c.pending = append(c.pending[:i], c.pending[i+1:]...)
					break
				}
			}
			c.mu.Unlock()
			h.fail(e)
			return true
		}
		if ch, ok := c.listReqs[requestID]; ok {
			delete(c.listReqs, requestID)
			c.mu.Unlock()
			select {
			case ch <- listResult{err: e}:
			default:
			}
			return true
		}
		for key, sub := range c.subscribers {
			if sub.subscribeID == requestID {
				delete(c.subscribers, key)
				c.mu.Unlock()
				sub.close(e)
				return true
			}
		}
	}
	if jobID != "" {
		if h, ok := c.handles[jobID]; ok {
			c.mu.Unlock()
			h.fail(e)
			return true
		}
	}
	c.mu.Unlock()
	return false
}

func (c *Client) dispatch(env arcp.Envelope) {
	switch env.Type {
	case messages.TypeSessionPing:
		c.handlePing(env)
	case messages.TypeSessionPong:
		// best-effort; nothing to do
	case messages.TypeSessionError:
		var serr messages.SessionError
		_ = env.DecodePayload(&serr)
		e := &arcp.Error{Code: serr.Code, Message: serr.Message, Retryable: serr.Retryable, Details: serr.Details}
		// Per-request errors (unknown agent, denied subscribe, unknown
		// cancel, …) are correlated to their originating call and must
		// not tear down the whole session. Only fail everything for a
		// genuinely session-fatal error that we cannot correlate.
		jobID := serr.JobID
		if jobID == "" {
			jobID = env.JobID
		}
		if c.routeError(serr.RequestID, jobID, e) {
			break
		}
		if isSessionFatal(serr.Code) {
			c.failAll(e)
		}
	case messages.TypeSessionJobs:
		c.handleSessionJobs(env)
	case messages.TypeJobAccepted:
		c.handleJobAccepted(env)
	case messages.TypeJobEvent:
		c.handleJobEvent(env)
	case messages.TypeJobResult, messages.TypeJobError:
		c.handleJobTerminal(env)
	case messages.TypeJobSubscribed:
		c.handleJobSubscribed(env)
	}
	c.maybeAck()
}

func (c *Client) handlePing(env arcp.Envelope) {
	var ping messages.SessionPing
	_ = env.DecodePayload(&ping)
	out, err := arcp.NewEnvelope(messages.TypeSessionPong, &messages.SessionPong{
		PingNonce: ping.Nonce,
	})
	if err != nil {
		return
	}
	out.SessionID = c.sessionID
	_ = c.transport.Send(c.ctx, out)
}

func (c *Client) handleSessionJobs(env arcp.Envelope) {
	var jobs messages.SessionJobs
	if err := env.DecodePayload(&jobs); err != nil {
		return
	}
	c.mu.Lock()
	ch, ok := c.listReqs[jobs.RequestID]
	if ok {
		delete(c.listReqs, jobs.RequestID)
	}
	c.mu.Unlock()
	if ok {
		select {
		case ch <- listResult{jobs: &jobs}:
		default:
		}
	}
}

// isSessionFatal reports whether a session.error code should terminate
// the whole session when it cannot be correlated to a specific request.
func isSessionFatal(code arcp.ErrorCode) bool {
	switch code {
	case arcp.CodeUnauthenticated, arcp.CodeHeartbeatLost:
		return true
	default:
		return false
	}
}

func (c *Client) handleJobAccepted(env arcp.Envelope) {
	var acc messages.JobAccepted
	if err := env.DecodePayload(&acc); err != nil {
		return
	}
	c.mu.Lock()
	var h *JobHandle
	if existing, ok := c.handles[acc.JobID]; ok {
		h = existing
	} else if len(c.pending) > 0 {
		h = c.pending[0]
		c.pending = c.pending[1:]
		c.handles[acc.JobID] = h
		if h.submitID != "" {
			delete(c.pendingByID, h.submitID)
		}
	}
	c.mu.Unlock()
	if h != nil {
		h.accept(acc)
	}
}

func (c *Client) handleJobEvent(env arcp.Envelope) {
	var ev messages.JobEvent
	if err := env.DecodePayload(&ev); err != nil {
		return
	}
	c.mu.RLock()
	h := c.handles[env.JobID]
	c.mu.RUnlock()
	if h != nil {
		h.pushEvent(ev)
	}
	c.mu.RLock()
	subs := append([]*Subscription(nil), c.subscribersFor(env.JobID)...)
	c.mu.RUnlock()
	for _, s := range subs {
		s.push(ev)
	}
}

func (c *Client) subscribersFor(jobID string) []*Subscription {
	var out []*Subscription
	for _, s := range c.subscribers {
		if s.jobID == jobID {
			out = append(out, s)
		}
	}
	return out
}

// removeSubscriber drops the subscriber under key, releasing references
// so future events for the same job no longer route through it. Safe to
// call multiple times.
func (c *Client) removeSubscriber(key string) {
	if key == "" {
		return
	}
	c.mu.Lock()
	delete(c.subscribers, key)
	c.mu.Unlock()
}

// removePending drops h from the pending FIFO; called on every error
// path of Submit so a stale handle cannot soak up a later acceptance.
func (c *Client) removePending(h *JobHandle) {
	c.mu.Lock()
	for i, ph := range c.pending {
		if ph == h {
			c.pending = append(c.pending[:i], c.pending[i+1:]...)
			break
		}
	}
	if h.submitID != "" {
		delete(c.pendingByID, h.submitID)
	}
	c.mu.Unlock()
}

func (c *Client) handleJobTerminal(env arcp.Envelope) {
	c.mu.RLock()
	h := c.handles[env.JobID]
	subs := append([]*Subscription(nil), c.subscribersFor(env.JobID)...)
	c.mu.RUnlock()
	switch env.Type {
	case messages.TypeJobResult:
		var r messages.JobResult
		_ = env.DecodePayload(&r)
		if h != nil {
			h.finish(&r, nil)
		}
		for _, s := range subs {
			s.close(nil)
		}
	case messages.TypeJobError:
		var jerr messages.JobError
		_ = env.DecodePayload(&jerr)
		e := &arcp.Error{Code: jerr.Code, Message: jerr.Message, Retryable: jerr.Retryable, Details: jerr.Details}
		if h != nil {
			h.finish(nil, e)
		}
		for _, s := range subs {
			s.close(e)
		}
	}
}

func (c *Client) handleJobSubscribed(env arcp.Envelope) {
	var sub messages.JobSubscribed
	if err := env.DecodePayload(&sub); err != nil {
		return
	}
	c.mu.RLock()
	subs := append([]*Subscription(nil), c.subscribersFor(sub.JobID)...)
	c.mu.RUnlock()
	for _, s := range subs {
		s.acknowledged(sub)
	}
}

func (c *Client) maybeAck() {
	if !c.HasFeature("ack") || c.opts.AutoAckWindow == 0 {
		return
	}
	current := c.highSeq.Load()
	last := c.lastAck.Load()
	if current-last < c.opts.AutoAckWindow {
		return
	}
	if c.lastAck.CompareAndSwap(last, current) {
		go c.sendAck(current)
	}
}

func (c *Client) sendAck(seq uint64) {
	body := messages.SessionAck{LastProcessedSeq: seq}
	env, err := arcp.NewEnvelope(messages.TypeSessionAck, &body)
	if err != nil {
		return
	}
	env.SessionID = c.sessionID
	_ = c.transport.Send(c.ctx, env)
}
