package client

import (
	"context"
	"sync"

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

	mu       sync.Mutex
	events   chan messages.JobEvent
	doneCh   chan struct{}
	err      error
	ack      *messages.JobSubscribed
	ackCh    chan messages.JobSubscribed
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
	sub := &Subscription{
		client: c,
		jobID:  jobID,
		events: make(chan messages.JobEvent, 128),
		doneCh: make(chan struct{}),
		ackCh:  make(chan messages.JobSubscribed, 1),
	}
	c.mu.Lock()
	c.subscribers[jobID+":"+arcp.NewULID()] = sub
	c.mu.Unlock()
	if err := c.transport.Send(ctx, env); err != nil {
		return nil, err
	}
	select {
	case ack := <-sub.ackCh:
		sub.ack = &ack
		return sub, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.ctx.Done():
		return nil, arcp.ErrInternalError.WithMessage("client closed")
	}
}

// JobID returns the subscribed job id.
func (s *Subscription) JobID() string { return s.jobID }

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

// Close detaches the subscription.
func (s *Subscription) Close(ctx context.Context) error {
	body := messages.JobUnsubscribe{JobID: s.jobID}
	env, err := arcp.NewEnvelope(messages.TypeJobUnsubscribe, &body)
	if err != nil {
		return err
	}
	env.SessionID = s.client.sessionID
	env.JobID = s.jobID
	_ = s.client.transport.Send(ctx, env)
	s.close(nil)
	return nil
}

func (s *Subscription) acknowledged(payload messages.JobSubscribed) {
	select {
	case s.ackCh <- payload:
	default:
	}
}

func (s *Subscription) push(ev messages.JobEvent) {
	select {
	case s.events <- ev:
	default:
	}
}

func (s *Subscription) close(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil || isClosed(s.doneCh) {
		return
	}
	s.err = err
	close(s.events)
	close(s.doneCh)
}

func isClosed(ch chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}
