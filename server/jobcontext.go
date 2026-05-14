package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"strings"
	"sync"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
	"github.com/agentruntimecontrolprotocol/go-sdk/messages"
)

// JobContext is the agent-facing surface inside an AgentFunc. It is
// the only sanctioned way to emit events, validate lease ops, debit
// budget counters, and stream chunked results.
type JobContext struct {
	job *Job

	streamed *streamedResult
	streamMu sync.Mutex

	budgetMu       sync.Mutex
	lastRemainEmit map[arcp.Currency]float64
}

// JobID returns the owning job id.
func (jc *JobContext) JobID() string { return jc.job.id }

// SessionID returns the owning session id.
func (jc *JobContext) SessionID() string { return jc.job.session.id }

// TraceID returns the trace identifier propagated from job.submit.
func (jc *JobContext) TraceID() string { return jc.job.traceID }

// Context returns the cancellable context bound to this job's
// lifecycle.
func (jc *JobContext) Context() context.Context { return jc.job.ctx }

// Lease returns a snapshot of the job's lease grant.
func (jc *JobContext) Lease() arcp.Lease { return jc.job.lease.Lease() }

// Budget returns a snapshot of remaining per-currency budget.
func (jc *JobContext) Budget() map[arcp.Currency]float64 {
	return jc.job.lease.Budget()
}

// ValidateLeaseOp checks that (cap, target) is permitted under the
// lease at the current clock time.
func (jc *JobContext) ValidateLeaseOp(cap arcp.Capability, target string) error {
	return jc.job.lease.ValidateOp(jc.job.session.srv.opts.Clock.Now(), cap, target)
}

// Log emits a "log" job event.
func (jc *JobContext) Log(level slog.Level, msg string, attrs ...slog.Attr) {
	body := messages.LogBody{Level: level.String(), Message: msg}
	_ = attrs
	jc.emitEvent(messages.KindLog, body)
}

// Thought emits a "thought" job event.
func (jc *JobContext) Thought(text string) {
	jc.emitEvent(messages.KindThought, messages.ThoughtBody{Text: text})
}

// ToolCall emits a "tool_call" event and returns the generated call_id
// the agent should reuse in the matching ToolResult / ToolError.
func (jc *JobContext) ToolCall(tool string, args any) string {
	raw, _ := json.Marshal(args)
	callID := arcp.NewCallID()
	jc.emitEvent(messages.KindToolCall, messages.ToolCallBody{
		Tool:   tool,
		Args:   raw,
		CallID: callID,
	})
	return callID
}

// ToolResult emits a successful "tool_result" event.
func (jc *JobContext) ToolResult(callID string, result any) {
	raw, _ := json.Marshal(result)
	jc.emitEvent(messages.KindToolResult, messages.ToolResultBody{
		CallID: callID,
		Result: raw,
	})
}

// ToolError emits a failed "tool_result" event.
func (jc *JobContext) ToolError(callID string, err error) {
	tool := &messages.ToolError{
		Code:      arcp.Code(err),
		Message:   err.Error(),
		Retryable: arcp.IsRetryable(err),
	}
	jc.emitEvent(messages.KindToolResult, messages.ToolResultBody{
		CallID: callID,
		Error:  tool,
	})
}

// Status emits a "status" event.
func (jc *JobContext) Status(phase, message string) {
	jc.emitEvent(messages.KindStatus, messages.StatusBody{Phase: phase, Message: message})
}

// Metric emits a "metric" event. If name begins with "cost." and unit
// matches a budgeted currency, the runtime debits the counter and may
// emit a follow-up cost.budget.remaining metric.
func (jc *JobContext) Metric(name string, value float64, unit string, dims map[string]string) {
	jc.emitEvent(messages.KindMetric, messages.MetricBody{
		Name:       name,
		Value:      value,
		Unit:       unit,
		Dimensions: dims,
	})
	if strings.HasPrefix(name, "cost.") && unit != "" && value >= 0 {
		cur := arcp.Currency(unit)
		remaining, err := jc.job.lease.Debit(cur, value)
		if err == nil && jc.job.lease.HasBudget() {
			jc.maybeEmitRemaining(cur, remaining)
		}
	}
}

func (jc *JobContext) maybeEmitRemaining(cur arcp.Currency, remaining float64) {
	jc.budgetMu.Lock()
	if jc.lastRemainEmit == nil {
		jc.lastRemainEmit = map[arcp.Currency]float64{}
	}
	last, ok := jc.lastRemainEmit[cur]
	jc.budgetMu.Unlock()
	initial := jc.job.lease.Initial()[cur]
	delta := math.Abs(last - remaining)
	if !ok || initial == 0 || delta/initial >= 0.05 || remaining <= 0 {
		jc.budgetMu.Lock()
		jc.lastRemainEmit[cur] = remaining
		jc.budgetMu.Unlock()
		jc.emitEvent(messages.KindMetric, messages.MetricBody{
			Name:  "cost.budget.remaining",
			Value: remaining,
			Unit:  string(cur),
		})
	}
}

// ArtifactRef emits an "artifact_ref" event.
func (jc *JobContext) ArtifactRef(uri, contentType string, byteSize uint64, sha256 string) {
	jc.emitEvent(messages.KindArtifactRef, messages.ArtifactRefBody{
		URI: uri, ContentType: contentType, ByteSize: byteSize, SHA256: sha256,
	})
}

// Progress emits a "progress" event.
func (jc *JobContext) Progress(current uint64, total uint64, units, message string) {
	jc.emitEvent(messages.KindProgress, messages.ProgressBody{
		Current: current, Total: total, Units: units, Message: message,
	})
}

// StreamResult opens a writer that emits result_chunk events. Close
// terminates the stream; the runtime emits the final job.result with
// result_id and result_size when the AgentFunc returns.
func (jc *JobContext) StreamResult(encoding string) (io.WriteCloser, error) {
	if encoding == "" {
		encoding = "utf8"
	}
	if encoding != "utf8" && encoding != "base64" {
		return nil, arcp.ErrInvalidRequest.WithMessage("encoding must be utf8 or base64")
	}
	jc.streamMu.Lock()
	defer jc.streamMu.Unlock()
	if jc.streamed != nil {
		return nil, arcp.ErrInvalidRequest.WithMessage("StreamResult called twice on the same job")
	}
	res := &streamedResult{
		jc:       jc,
		resultID: arcp.NewResultID(),
		encoding: encoding,
		max:      jc.job.session.srv.opts.MaxResultBytes,
	}
	jc.streamed = res
	return res, nil
}

// emitEvent allocates a session-scoped event_seq and pushes one
// job.event envelope.
func (jc *JobContext) emitEvent(kind string, body any) {
	raw, err := json.Marshal(body)
	if err != nil {
		return
	}
	ev := messages.JobEvent{
		Kind: kind,
		TS:   jc.job.session.srv.opts.Clock.Now(),
		Body: raw,
	}
	env, err := arcp.NewEnvelope(messages.TypeJobEvent, &ev)
	if err != nil {
		return
	}
	env.JobID = jc.job.id
	env.TraceID = jc.job.traceID
	env.EventSeq = jc.job.session.nextSeq()
	jc.job.session.send(env)
	jc.job.session.srv.fanoutEvent(jc.job.ctx, jc.job.id, env)
}

type streamedResult struct {
	jc       *JobContext
	resultID string
	encoding string
	mu       sync.Mutex
	seq      uint64
	size     uint64
	max      int64
	closed   bool
}

// Write emits one result_chunk event.
func (r *streamedResult) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return 0, arcp.ErrInvalidRequest.WithMessage("write to closed result stream")
	}
	if r.max > 0 && int64(r.size)+int64(len(p)) > r.max {
		return 0, arcp.ErrInternalError.WithMessage(fmt.Sprintf("streamed result exceeds %d bytes", r.max))
	}
	var data string
	switch r.encoding {
	case "utf8":
		data = string(p)
	case "base64":
		data = base64.StdEncoding.EncodeToString(p)
	}
	body := messages.ResultChunkBody{
		ResultID: r.resultID,
		ChunkSeq: r.seq,
		Data:     data,
		Encoding: r.encoding,
		More:     true,
	}
	r.jc.emitEvent(messages.KindResultChunk, body)
	r.seq++
	r.size += uint64(len(p))
	return len(p), nil
}

// Close finalises the stream by emitting a chunk with more=false.
func (r *streamedResult) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil
	}
	r.closed = true
	body := messages.ResultChunkBody{
		ResultID: r.resultID,
		ChunkSeq: r.seq,
		Data:     "",
		Encoding: r.encoding,
		More:     false,
	}
	r.jc.emitEvent(messages.KindResultChunk, body)
	return nil
}
