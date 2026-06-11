package client

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

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
	client   *Client
	id       string
	agent    string
	submitID string

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
		submitID: env.ID,
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
	// Serialize the submit handshake so c.pending is always in the
	// same order as the runtime's acceptance stream. Without this two
	// concurrent submits could land on the wire in one order, append
	// to c.pending in another, and cross-resolve.
	c.submitMu.Lock()
	c.mu.Lock()
	c.pending = append(c.pending, h)
	c.pendingByID[env.ID] = h
	c.mu.Unlock()
	if err := c.transport.Send(ctx, env); err != nil {
		c.removePending(h)
		c.submitMu.Unlock()
		return nil, err
	}
	select {
	case acc := <-accepted:
		h.id = acc.JobID
		c.submitMu.Unlock()
		return h, nil
	case <-h.doneCh:
		c.removePending(h)
		c.submitMu.Unlock()
		return nil, h.Err()
	case <-ctx.Done():
		c.removePending(h)
		c.submitMu.Unlock()
		return nil, ctx.Err()
	case <-c.ctx.Done():
		c.removePending(h)
		c.submitMu.Unlock()
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
//
// CollectChunks also drains the unfiltered Events() channel as a
// best-effort consumer so the lossless dispatcher does not stall when
// the caller is only interested in the assembled chunk bytes; the
// drained non-chunk events are discarded.
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
		case <-h.eventsCh:
			// Drain so the dispatcher does not block on a full events
			// channel while the caller only reads chunks. We can't
			// peek non-blocking inside this select; the receive is
			// the drain.
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-h.doneCh:
			// Drain any remaining buffered chunks (finish() closes
			// the channel; this loop walks to the close marker so
			// no chunks are lost when the handle terminates while
			// chunks were still in the buffer).
			for ch := range h.chunksCh {
				a, exists := by[ch.ResultID]
				if !exists {
					a = &chunkAccum{encoding: ch.Encoding, chunks: map[uint64]string{}}
					by[ch.ResultID] = a
				} else if a.encoding != ch.Encoding {
					return nil, arcp.ErrInvalidRequest.WithMessage("mixed encodings in result_chunk stream")
				}
				a.chunks[ch.ChunkSeq] = ch.Data
			}
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

// pushEvent forwards ev to consumers, preserving order and never
// silently dropping. By default Send blocks while the consumer is
// slow; the consumer is expected to drain. If
// Options.EventDeliveryTimeout is set, the handle closes with a
// structured overflow error if delivery does not complete within
// the timeout, so the caller observes a terminal state rather than
// missing chunks. Returns early if the handle has already
// terminated or the client context fires.
//
// result_chunk events also route to the dedicated chunks channel so
// CollectChunks sees every chunk in sequence.
func (h *JobHandle) pushEvent(ev messages.JobEvent) {
	if h.isDone() {
		return
	}
	if !h.deliver(h.eventsCh, ev) {
		return
	}
	if ev.Kind == messages.KindResultChunk {
		var body messages.ResultChunkBody
		if err := json.Unmarshal(ev.Body, &body); err == nil {
			h.deliverChunk(body)
		}
	}
}

func (h *JobHandle) isDone() bool {
	select {
	case <-h.doneCh:
		return true
	default:
		return false
	}
}

// deliver enqueues ev on ch, blocking with the configured timeout (or
// indefinitely when zero). Returns false if the handle was closed
// during delivery.
func (h *JobHandle) deliver(ch chan messages.JobEvent, ev messages.JobEvent) bool {
	timeout := h.client.opts.EventDeliveryTimeout
	if timeout <= 0 {
		select {
		case ch <- ev:
			return true
		case <-h.doneCh:
			return false
		case <-h.client.ctx.Done():
			return false
		}
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case ch <- ev:
		return true
	case <-h.doneCh:
		return false
	case <-h.client.ctx.Done():
		return false
	case <-timer.C:
		h.fail(arcp.ErrInternalError.WithMessage("job handle overflow: consumer did not drain within EventDeliveryTimeout"))
		return false
	}
}

// deliverChunk is the chunks-channel mirror of deliver, preserving
// chunk order for CollectChunks.
func (h *JobHandle) deliverChunk(body messages.ResultChunkBody) {
	timeout := h.client.opts.EventDeliveryTimeout
	if timeout <= 0 {
		select {
		case h.chunksCh <- body:
		case <-h.doneCh:
		case <-h.client.ctx.Done():
		}
		return
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case h.chunksCh <- body:
	case <-h.doneCh:
	case <-h.client.ctx.Done():
	case <-timer.C:
		h.fail(arcp.ErrInternalError.WithMessage("job handle overflow: consumer did not drain within EventDeliveryTimeout"))
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
