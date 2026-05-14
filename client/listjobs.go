package client

import (
	"context"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
	"github.com/agentruntimecontrolprotocol/go-sdk/messages"
)

// ListJobsRequest is the input to Client.ListJobs.
type ListJobsRequest struct {
	Filter messages.ListJobsFilter
	Limit  int
	Cursor string
}

// JobList is the response.
type JobList struct {
	Jobs       []messages.JobInfo
	NextCursor string
}

// ListJobs requests a read-only inventory.
func (c *Client) ListJobs(ctx context.Context, req ListJobsRequest) (*JobList, error) {
	if !c.HasFeature("list_jobs") {
		return nil, arcp.ErrInvalidRequest.WithMessage("list_jobs feature not negotiated")
	}
	body := messages.SessionListJobs{
		Filter: req.Filter,
		Limit:  req.Limit,
		Cursor: req.Cursor,
	}
	env, err := arcp.NewEnvelope(messages.TypeSessionListJobs, &body)
	if err != nil {
		return nil, err
	}
	env.SessionID = c.sessionID
	ch := make(chan *messages.SessionJobs, 1)
	c.mu.Lock()
	c.listReqs[env.ID] = ch
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.listReqs, env.ID)
		c.mu.Unlock()
	}()
	if err := c.transport.Send(ctx, env); err != nil {
		return nil, err
	}
	select {
	case resp := <-ch:
		return &JobList{Jobs: resp.Jobs, NextCursor: resp.NextCursor}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.ctx.Done():
		return nil, arcp.ErrInternalError.WithMessage("client closed")
	}
}

// Ack emits a session.ack with last_processed_seq.
func (c *Client) Ack(ctx context.Context, seq uint64) error {
	if !c.HasFeature("ack") {
		return arcp.ErrInvalidRequest.WithMessage("ack feature not negotiated")
	}
	body := messages.SessionAck{LastProcessedSeq: seq}
	env, err := arcp.NewEnvelope(messages.TypeSessionAck, &body)
	if err != nil {
		return err
	}
	env.SessionID = c.sessionID
	return c.transport.Send(ctx, env)
}
