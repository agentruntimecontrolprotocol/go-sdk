package client

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
	"github.com/agentruntimecontrolprotocol/go-sdk/messages"
)

// SubmitRequest is the input to Client.Submit.
type SubmitRequest struct {
	Agent            string
	Input            any
	LeaseRequest     arcp.Lease
	LeaseConstraints *messages.LeaseConstraints
	IdempotencyKey   string
	MaxRuntimeSec    int
	TraceID          string
}

// JobHandle is the client-side view of one submitted job.
type JobHandle struct {
	client *Client
	id     string
	agent  string

	mu             sync.Mutex
	accepted       *messages.JobAccepted
	result         *messages.JobResult
	err            error
	doneCh         chan struct{}
	eventsCh       chan messages.JobEvent
	chunksCh       chan messages.ResultChunkBody
	acceptObserver func(messages.JobAccepted)
}

// Submit emits a job.submit envelope and returns a handle. The
// returned handle has a pre-allocated job id; the runtime echoes it
// back in job.accepted, at which point the handle is "accepted".
func (c *Client) Submit(ctx context.Context, req SubmitRequest) (*JobHandle, error) {
	input, err := arcp.MarshalPayload(req.Input)
	if err != nil {
		return nil, err
	}
	payload := messages.JobSubmit{
		Agent:            req.Agent,
		Input:            input,
		LeaseRequest:     req.LeaseRequest,
		LeaseConstraints: req.LeaseConstraints,
		IdempotencyKey:   req.IdempotencyKey,
		MaxRuntimeSec:    req.MaxRuntimeSec,
	}
	env, err := arcp.NewEnvelope(messages.TypeJobSubmit, &payload)
	if err != nil {
		return nil, err
	}
	env.SessionID = c.sessionID
	env.TraceID = req.TraceID
	// The runtime allocates the job id. We use a deferred-allocation
	// strategy: this handle is indexed by the submit envelope id and
	// re-keyed when job.accepted arrives.
	h := &JobHandle{
		client:   c,
		agent:    req.Agent,
		doneCh:   make(chan struct{}),
		eventsCh: make(chan messages.JobEvent, 64),
		chunksCh: make(chan messages.ResultChunkBody, 64),
	}
	accepted := make(chan *messages.JobAccepted, 1)
	h.acceptObserver = func(acc messages.JobAccepted) {
		select {
		case accepted <- &acc:
		default:
		}
	}
	c.mu.Lock()
	c.pending = append(c.pending, h)
	c.mu.Unlock()
	if err := c.transport.Send(ctx, env); err != nil {
		return nil, err
	}
	select {
	case acc := <-accepted:
		h.id = acc.JobID
		return h, nil
	case <-h.doneCh:
		return nil, h.Err()
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.ctx.Done():
		return nil, errors.New("client closed before job.accepted arrived")
	}
}

// ID returns the runtime-assigned job id once accepted, else "".
func (h *JobHandle) ID() string { return h.id }

// Agent returns the requested agent identifier.
func (h *JobHandle) Agent() string { return h.agent }

// Accepted returns the job.accepted payload.
func (h *JobHandle) Accepted() *messages.JobAccepted { return h.accepted }

// Done returns a channel closed when the job reaches a terminal
// state.
func (h *JobHandle) Done() <-chan struct{} { return h.doneCh }

// Events returns the live event channel. It is closed when the job
// reaches a terminal state.
func (h *JobHandle) Events() <-chan messages.JobEvent { return h.eventsCh }

// Chunks returns the result_chunk-only event channel. Closed when the
// job terminates.
func (h *JobHandle) Chunks() <-chan messages.ResultChunkBody { return h.chunksCh }

// Err returns the terminal error, if any.
func (h *JobHandle) Err() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.err
}

// Result returns the terminal job.result payload, if any.
func (h *JobHandle) Result() *messages.JobResult {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.result
}

// Wait blocks until the job terminates or ctx is cancelled.
func (h *JobHandle) Wait(ctx context.Context) (*messages.JobResult, error) {
	select {
	case <-h.doneCh:
		return h.Result(), h.Err()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Cancel sends job.cancel.
func (h *JobHandle) Cancel(ctx context.Context, reason string) error {
	body := messages.JobCancel{Reason: reason}
	env, err := arcp.NewEnvelope(messages.TypeJobCancel, &body)
	if err != nil {
		return err
	}
	env.SessionID = h.client.sessionID
	env.JobID = h.id
	return h.client.transport.Send(ctx, env)
}

// chunkAccum buffers chunks for one result_id.
type chunkAccum struct {
	encoding string
	chunks   map[uint64]string
}

// CollectChunks reads chunks until the stream terminates and returns
// the assembled bytes by result_id. Returns an error if encodings are
// mixed or chunks arrive out of order.
func (h *JobHandle) CollectChunks(ctx context.Context) ([]byte, error) {
	by := map[string]*chunkAccum{}
	for {
		select {
		case ch, ok := <-h.chunksCh:
			if !ok {
				return assembleChunks(by)
			}
			a, exists := by[ch.ResultID]
			if !exists {
				a = &chunkAccum{encoding: ch.Encoding, chunks: map[uint64]string{}}
				by[ch.ResultID] = a
			} else if a.encoding != ch.Encoding {
				return nil, arcp.ErrInvalidRequest.WithMessage("mixed encodings in result_chunk stream")
			}
			a.chunks[ch.ChunkSeq] = ch.Data
			if !ch.More {
				return assembleChunks(by)
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-h.doneCh:
			return assembleChunks(by)
		}
	}
}

func assembleChunks(by map[string]*chunkAccum) ([]byte, error) {
	if len(by) == 0 {
		return nil, nil
	}
	if len(by) > 1 {
		return nil, arcp.ErrInvalidRequest.WithMessage("multiple result_ids in stream")
	}
	for _, a := range by {
		seqs := make([]uint64, 0, len(a.chunks))
		for k := range a.chunks {
			seqs = append(seqs, k)
		}
		sort.Slice(seqs, func(i, j int) bool { return seqs[i] < seqs[j] })
		var out []byte
		for _, s := range seqs {
			data := a.chunks[s]
			switch a.encoding {
			case "utf8":
				out = append(out, []byte(data)...)
			case "base64":
				dec, err := base64.StdEncoding.DecodeString(data)
				if err != nil {
					return nil, fmt.Errorf("decode chunk %d: %w", s, err)
				}
				out = append(out, dec...)
			}
		}
		return out, nil
	}
	return nil, nil
}

// accept marks h accepted.
func (h *JobHandle) accept(acc messages.JobAccepted) {
	h.mu.Lock()
	if h.accepted == nil {
		cp := acc
		h.accepted = &cp
	}
	obs := h.acceptObserver
	h.mu.Unlock()
	if obs != nil {
		obs(acc)
	}
}

// pushEvent forwards ev to consumers; result_chunk events also route
// to the chunks channel.
func (h *JobHandle) pushEvent(ev messages.JobEvent) {
	select {
	case h.eventsCh <- ev:
	default:
	}
	if ev.Kind == messages.KindResultChunk {
		var body messages.ResultChunkBody
		if err := json.Unmarshal(ev.Body, &body); err == nil {
			select {
			case h.chunksCh <- body:
			default:
			}
		}
	}
}

// finish marks h done with either a result or error.
func (h *JobHandle) finish(r *messages.JobResult, err error) {
	h.mu.Lock()
	if h.result == nil && h.err == nil {
		h.result = r
		h.err = err
		close(h.doneCh)
		close(h.eventsCh)
		close(h.chunksCh)
	}
	h.mu.Unlock()
}

// fail forces a terminal error without a job.error envelope.
func (h *JobHandle) fail(err error) {
	h.finish(nil, err)
}
