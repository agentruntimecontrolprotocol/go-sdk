package client

import (
	"context"
	"sync"
	"time"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
	"github.com/agentruntimecontrolprotocol/go-sdk/messages"
)

// SubscribeOptions configures a Client.Subscribe call.
type SubscribeOptions struct {
	FromEventSeq uint64
	History      bool
}

// Subscription is the client-side view of a job.subscribe attachment.
type Subscription struct {
	client *Client
	jobID  string
	// key is the registration key in client.subscribers; held so Close
	// can remove the entry. Empty for not-yet-registered subscriptions.
	key string
	// subscribeID is the envelope id of the job.subscribe request, used
	// to correlate a denial session.error back to this subscription.
	subscribeID string

	mu        sync.Mutex
	events    chan messages.JobEvent
	doneCh    chan struct{}
	err       error
	ack       *messages.JobSubscribed
	ackCh     chan messages.JobSubscribed
	closeOnce sync.Once
	// pushWG tracks in-flight push() calls so close() can wait for them
	// to drain before closing s.events. Without this, push() can be
	// mid-send on s.events while close() concurrently closes it — a
	// race the race detector flags and that can panic with
	// "send on closed channel".
	pushWG sync.WaitGroup
}

// Subscribe attaches the current session to jobID. The runtime
// validates principal authority before returning a subscription
// handle.
func (c *Client) Subscribe(ctx context.Context, jobID string, opts SubscribeOptions) (*Subscription, error) {
	if !c.HasFeature("subscribe") {
		return nil, arcp.ErrInvalidRequest.WithMessage("subscribe feature not negotiated")
	}
	body := messages.JobSubscribe{
		JobID:        jobID,
		FromEventSeq: opts.FromEventSeq,
		History:      opts.History,
	}
	env, err := arcp.NewEnvelope(messages.TypeJobSubscribe, &body)
	if err != nil {
		return nil, err
	}
	env.SessionID = c.sessionID
	env.JobID = jobID
	key := jobID + ":" + arcp.NewULID()
	sub := &Subscription{
		client:      c,
		jobID:       jobID,
		key:         key,
		subscribeID: env.ID,
		events:      make(chan messages.JobEvent, 128),
		doneCh:      make(chan struct{}),
		ackCh:       make(chan messages.JobSubscribed, 1),
	}
	c.mu.Lock()
	c.subscribers[key] = sub
	c.mu.Unlock()
	if err := c.transport.Send(ctx, env); err != nil {
		c.removeSubscriber(key)
		return nil, err
	}
	select {
	case ack := <-sub.ackCh:
		sub.ack = &ack
		return sub, nil
	case <-sub.doneCh:
		// The runtime denied the subscription (PERMISSION_DENIED,
		// JOB_NOT_FOUND, …): routeError closed the subscription with
		// the correlated error instead of ever feeding ackCh.
		c.removeSubscriber(key)
		if err := sub.Err(); err != nil {
			return nil, err
		}
		return nil, arcp.ErrInternalError.WithMessage("subscription rejected")
	case <-ctx.Done():
		c.removeSubscriber(key)
		return nil, ctx.Err()
	case <-c.ctx.Done():
		c.removeSubscriber(key)
		return nil, arcp.ErrInternalError.WithMessage("client closed")
	}
}

// JobID returns the subscribed job id.
func (s *Subscription) JobID() string { return s.jobID }

// CurrentStatus returns the job status reported in job.subscribed at
// attach time. Live updates arrive on the event stream as status
// events.
func (s *Subscription) CurrentStatus() string {
	if s.ack == nil {
		return ""
	}
	return s.ack.CurrentStatus
}

// Agent returns the resolved "name@version" agent reported in
// job.subscribed.
func (s *Subscription) Agent() string {
	if s.ack == nil {
		return ""
	}
	return s.ack.Agent
}

// Lease returns the effective lease reported in job.subscribed.
func (s *Subscription) Lease() arcp.Lease {
	if s.ack == nil {
		return nil
	}
	return s.ack.Lease
}

// ParentJobID returns the parent job id reported in job.subscribed,
// when the subscribed job is a delegated child. Empty otherwise.
func (s *Subscription) ParentJobID() string {
	if s.ack == nil {
		return ""
	}
	return s.ack.ParentJobID
}

// TraceID returns the trace id reported in job.subscribed.
func (s *Subscription) TraceID() string {
	if s.ack == nil {
		return ""
	}
	return s.ack.TraceID
}

// SubscribedFrom returns the session-scoped event_seq at which the
// runtime started feeding this subscription. Useful for clients that
// later need to filter or replay.
func (s *Subscription) SubscribedFrom() uint64 {
	if s.ack == nil {
		return 0
	}
	return s.ack.SubscribedFrom
}

// Replayed reports whether the runtime replayed buffered history
// before live events began.
func (s *Subscription) Replayed() bool {
	if s.ack == nil {
		return false
	}
	return s.ack.Replayed
}

// Events returns the live event channel.
func (s *Subscription) Events() <-chan messages.JobEvent { return s.events }

// Done returns a channel closed when the subscription ends.
func (s *Subscription) Done() <-chan struct{} { return s.doneCh }

// Err returns the terminal subscription error, if any.
func (s *Subscription) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

// Close detaches the subscription. Close is idempotent: subsequent
// calls return nil and do not re-send job.unsubscribe.
func (s *Subscription) Close(ctx context.Context) error {
	var sendErr error
	s.closeOnce.Do(func() {
		body := messages.JobUnsubscribe{JobID: s.jobID}
		env, err := arcp.NewEnvelope(messages.TypeJobUnsubscribe, &body)
		if err != nil {
			sendErr = err
			return
		}
		env.SessionID = s.client.sessionID
		env.JobID = s.jobID
		// Surface the unsubscribe send failure: a dropped envelope means
		// the runtime keeps streaming to a dead subscriber.
		sendErr = s.client.transport.Send(ctx, env)
		if s.key != "" {
			s.client.removeSubscriber(s.key)
		}
		s.close(nil)
	})
	return sendErr
}

func (s *Subscription) acknowledged(payload messages.JobSubscribed) {
	select {
	case s.ackCh <- payload:
	default:
	}
}

// push enqueues ev for the consumer. It blocks while the consumer is
// slow rather than silently dropping the envelope, but returns
// immediately if the subscription is already closed or the client's
// context fires. If the consumer never drains, push closes the
// subscription with a structured overflow error after the configured
// EventDeliveryTimeout. The default ordering and result_chunk
// guarantees promised by the README are preserved.
func (s *Subscription) push(ev messages.JobEvent) {
	// Register with the close barrier before checking isClosed so a
	// concurrent close() will Wait() for us to finish the send.
	s.pushWG.Add(1)
	defer s.pushWG.Done()
	if s.isClosed() {
		return
	}
	timeout := s.client.opts.EventDeliveryTimeout
	if timeout <= 0 {
		// Block indefinitely (subject to client ctx); the consumer is
		// expected to drain. This matches the "lossless" contract.
		select {
		case s.events <- ev:
		case <-s.doneCh:
		case <-s.client.ctx.Done():
		}
		return
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case s.events <- ev:
	case <-s.doneCh:
	case <-s.client.ctx.Done():
	case <-timer.C:
		s.close(arcp.ErrInternalError.WithMessage("subscription overflow: consumer did not drain within EventDeliveryTimeout"))
		if s.key != "" {
			s.client.removeSubscriber(s.key)
		}
	}
}

func (s *Subscription) isClosed() bool {
	select {
	case <-s.doneCh:
		return true
	default:
		return false
	}
}

func (s *Subscription) close(err error) {
	s.mu.Lock()
	if s.err != nil || isClosed(s.doneCh) {
		s.mu.Unlock()
		return
	}
	s.err = err
	// Close doneCh first so any in-flight push() unblocks via its
	// select case, then wait for those pushes to exit before closing
	// the events channel.
	close(s.doneCh)
	s.mu.Unlock()
	s.pushWG.Wait()
	close(s.events)
}

func isClosed(ch chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}
