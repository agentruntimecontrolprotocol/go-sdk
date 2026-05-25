package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
	"github.com/agentruntimecontrolprotocol/go-sdk/internal/clock"
	"github.com/agentruntimecontrolprotocol/go-sdk/internal/lease"
	"github.com/agentruntimecontrolprotocol/go-sdk/messages"
)

// Job is the runtime-side representation of an accepted job.
type Job struct {
	id        string
	agent     string
	principal string
	session   *session
	fn        AgentFunc
	input     json.RawMessage
	traceID   string
	createdAt time.Time

	parent *Job

	lease *lease.State
	creds []messages.Credential

	ctx    context.Context
	cancel context.CancelFunc

	mu         sync.Mutex
	credsMu    sync.Mutex
	revokeOnce sync.Once
	statusV    string
	terminal   bool
	cancelRe   string

	expiryTimer clock.Timer

	// streamCount limits at most one StreamResult per job.
	streamCount int
}

func newJob(s *session, canonicalAgent string, req messages.JobSubmit, fn AgentFunc, traceID string) *Job {
	// Jobs are rooted in the server's lifetime context so they survive
	// transport disconnects; only Server.Close, job.cancel,
	// max_runtime_sec, lease expiry, or terminal completion ends them.
	ctx, cancel := context.WithCancel(s.srv.lifeCtx)
	id := arcp.NewJobID()
	var expiresAt *time.Time
	if req.LeaseConstraints != nil && req.LeaseConstraints.ExpiresAt != nil {
		t := req.LeaseConstraints.ExpiresAt.UTC()
		expiresAt = &t
	}
	st := lease.NewState(req.LeaseRequest, expiresAt)
	if traceID == "" {
		traceID = arcp.NewTraceID()
	}
	j := &Job{
		id:        id,
		agent:     canonicalAgent,
		principal: s.principal,
		session:   s,
		fn:        fn,
		input:     req.Input,
		traceID:   traceID,
		createdAt: s.srv.opts.Clock.Now(),
		lease:     st,
		ctx:       ctx,
		cancel:    cancel,
		statusV:   messages.StatusPending,
	}
	if expiresAt != nil {
		d := expiresAt.Sub(s.srv.opts.Clock.Now())
		if d > 0 {
			j.expiryTimer = s.srv.opts.Clock.AfterFunc(d, func() {
				j.expireLease()
			})
		} else {
			go j.expireLease()
		}
	}
	if req.MaxRuntimeSec > 0 {
		d := time.Duration(req.MaxRuntimeSec) * time.Second
		_ = s.srv.opts.Clock.AfterFunc(d, func() {
			j.timeout()
		})
	}
	return j
}

// ID returns the job identifier.
func (j *Job) ID() string { return j.id }

// Agent returns the resolved "name@version" string.
func (j *Job) Agent() string { return j.agent }

// Principal returns the submitting principal.
func (j *Job) Principal() string { return j.principal }

// Lease returns a snapshot of the job's lease.
func (j *Job) Lease() arcp.Lease { return j.lease.Lease() }

// status returns the FSM state as a wire string.
func (j *Job) status() string {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.statusV
}

func (j *Job) setStatus(s string) {
	j.mu.Lock()
	j.statusV = s
	j.mu.Unlock()
}

func (j *Job) markTerminal(final string) bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.terminal {
		return false
	}
	j.terminal = true
	j.statusV = final
	return true
}

func (j *Job) snapshot() messages.JobInfo {
	return messages.JobInfo{
		JobID:        j.id,
		Agent:        j.agent,
		Status:       j.status(),
		Lease:        j.lease.Lease(),
		CreatedAt:    j.createdAt,
		TraceID:      j.traceID,
		LastEventSeq: j.session.currentSeq(),
	}
}

func (j *Job) attachCredentials(creds []messages.Credential) {
	j.credsMu.Lock()
	defer j.credsMu.Unlock()
	j.creds = append(j.creds, creds...)
}

func (j *Job) outstandingCredentials() []messages.Credential {
	j.credsMu.Lock()
	defer j.credsMu.Unlock()
	return append([]messages.Credential(nil), j.creds...)
}

func (j *Job) replaceCredentialValue(id, newValue string) bool {
	j.credsMu.Lock()
	defer j.credsMu.Unlock()
	for i := range j.creds {
		if j.creds[i].ID == id {
			j.creds[i].Value = newValue
			return true
		}
	}
	return false
}

func (j *Job) cancelWithReason(reason string) {
	j.mu.Lock()
	j.cancelRe = reason
	j.mu.Unlock()
	j.cancel()
}

func (j *Job) expireLease() {
	if !j.markTerminal(messages.StatusError) {
		return
	}
	j.emitTerminalError(arcp.CodeLeaseExpired, "lease expired during execution")
	j.revokeAll()
	j.cancel()
}

func (j *Job) timeout() {
	if !j.markTerminal(messages.StatusTimedOut) {
		return
	}
	j.emitTerminalError(arcp.CodeTimeout, "job exceeded max_runtime_sec")
	j.revokeAll()
	j.cancel()
}

func (j *Job) emitTerminalError(code arcp.ErrorCode, msg string) {
	final := messages.StatusError
	if code == arcp.CodeTimeout {
		final = messages.StatusTimedOut
	}
	body := messages.JobError{
		FinalStatus: final,
		Code:        code,
		Message:     msg,
		Retryable:   false,
	}
	j.emitTerminal(messages.TypeJobError, &body)
}

// emitTerminal sends env-typed terminal envelope on the session.
func (j *Job) emitTerminal(typ string, payload any) {
	env, err := arcp.NewEnvelope(typ, payload)
	if err != nil {
		return
	}
	env.JobID = j.id
	env.TraceID = j.traceID
	env.EventSeq = j.session.nextSeq()
	j.session.send(env)
	j.session.srv.fanoutEvent(j.ctx, j.id, env)
}

// run executes the agent body inside the job's context.
func (j *Job) run() {
	if j.expiryTimer != nil {
		defer j.expiryTimer.Stop()
	}
	j.setStatus(messages.StatusRunning)
	jc := &JobContext{job: j}
	var (
		output any
		err    error
	)
	func() {
		defer func() {
			if rec := recover(); rec != nil {
				err = fmt.Errorf("agent panic: %v\n%s", rec, debug.Stack())
			}
		}()
		output, err = j.fn(j.ctx, j.input, jc)
	}()
	if errors.Is(err, context.Canceled) || j.ctx.Err() != nil {
		// Was the job cancelled by the submitter, or did we hit a
		// terminal error already?
		if j.markTerminal(messages.StatusCancelled) {
			j.mu.Lock()
			reason := j.cancelRe
			j.mu.Unlock()
			body := messages.JobError{
				FinalStatus: messages.StatusCancelled,
				Code:        arcp.CodeCancelled,
				Message:     reason,
				Retryable:   false,
			}
			j.emitTerminal(messages.TypeJobError, &body)
			j.revokeAll()
		}
		return
	}
	if err != nil {
		if j.markTerminal(messages.StatusError) {
			code := arcp.Code(err)
			body := messages.JobError{
				FinalStatus: messages.StatusError,
				Code:        code,
				Message:     err.Error(),
				Retryable:   arcp.IsRetryable(err),
			}
			j.emitTerminal(messages.TypeJobError, &body)
			j.revokeAll()
		}
		return
	}
	// Success path. If a stream was opened, the writer has already
	// finalised the result; otherwise emit inline.
	if jc.streamed != nil {
		if !j.markTerminal(messages.StatusSuccess) {
			return
		}
		final := messages.JobResult{
			FinalStatus: messages.StatusSuccess,
			ResultID:    jc.streamed.resultID,
			ResultSize:  jc.streamed.size,
		}
		j.emitTerminal(messages.TypeJobResult, &final)
		j.revokeAll()
		return
	}
	if !j.markTerminal(messages.StatusSuccess) {
		return
	}
	body := messages.JobResult{
		FinalStatus: messages.StatusSuccess,
	}
	if output != nil {
		raw, mErr := json.Marshal(output)
		if mErr != nil {
			body.FinalStatus = messages.StatusError
			ebody := messages.JobError{
				FinalStatus: messages.StatusError,
				Code:        arcp.CodeInternalError,
				Message:     "marshal result: " + mErr.Error(),
				Retryable:   true,
			}
			j.emitTerminal(messages.TypeJobError, &ebody)
			j.revokeAll()
			return
		}
		body.Output = raw
	}
	j.emitTerminal(messages.TypeJobResult, &body)
	j.revokeAll()
}

func (j *Job) revokeAll() {
	if j.session.srv.opts.Provisioner == nil {
		return
	}
	j.revokeOnce.Do(func() {
		creds := j.outstandingCredentials()
		for _, cred := range creds {
			j.revokeCredential(cred.ID)
		}
	})
}

func (j *Job) revokeCredential(id string) {
	backoff := []time.Duration{50 * time.Millisecond, 250 * time.Millisecond, time.Second}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var err error
	for attempt := 0; attempt < len(backoff)+1; attempt++ {
		err = j.session.srv.opts.Provisioner.Revoke(ctx, id)
		if err == nil {
			return
		}
		if attempt == len(backoff) {
			break
		}
		timer := time.NewTimer(backoff[attempt])
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			break
		}
	}
	j.session.srv.opts.Logger.Error("credential revocation failed", "job_id", j.id, "credential_id", id, "err", err)
}
