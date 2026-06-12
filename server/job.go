package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
	"github.com/agentruntimecontrolprotocol/go-sdk/credentials"
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

	expiryTimer  clock.Timer
	runtimeTimer clock.Timer

	// lastEventSeq is the highest event_seq this job has emitted. It is
	// reported as JobInfo.LastEventSeq (a per-job high-water mark) rather
	// than reading the session-wide counter off a possibly-stale session
	// pointer (#69).
	lastEventSeq atomic.Uint64

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
		j.runtimeTimer = s.srv.opts.Clock.AfterFunc(d, func() {
			j.timeout()
		})
	}
	return j
}

// discard tears down a job that was created but rejected before it ever
// started running (duplicate idempotency key, store/provisioner/envelope
// error, id collision). It marks the job terminal so a late expiry or
// max-runtime timer fire becomes a no-op (no spurious job.error for a
// job the client never saw accepted), stops both timers, and releases
// the job context so rejected submits do not leak lifeCtx children.
func (j *Job) discard() {
	j.markTerminal(messages.StatusError)
	if j.expiryTimer != nil {
		j.expiryTimer.Stop()
	}
	if j.runtimeTimer != nil {
		j.runtimeTimer.Stop()
	}
	j.cancel()
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
	// Never overwrite a terminal status with a non-terminal one (#79):
	// an already-expired lease can mark the job terminal before run()
	// starts.
	if !j.terminal {
		j.statusV = s
	}
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
		LastEventSeq: j.lastEventSeq.Load(),
	}
}

// recordSeq advances the per-job event_seq high-water mark.
func (j *Job) recordSeq(seq uint64) {
	for {
		cur := j.lastEventSeq.Load()
		if seq <= cur || j.lastEventSeq.CompareAndSwap(cur, seq) {
			return
		}
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

// replaceCredentialValue swaps the stored value for credential id and
// returns the prior value plus whether the credential was found.
func (j *Job) replaceCredentialValue(id, newValue string) (string, bool) {
	j.credsMu.Lock()
	defer j.credsMu.Unlock()
	for i := range j.creds {
		if j.creds[i].ID == id {
			prior := j.creds[i].Value
			j.creds[i].Value = newValue
			return prior, true
		}
	}
	return "", false
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
	// §14: log lease expirations for audit.
	var expiresAt any
	if t := j.lease.ExpiresAt(); t != nil {
		expiresAt = t.Format(time.RFC3339)
	}
	j.session.srv.opts.Logger.Info("lease expired",
		"job_id", j.id, "principal", j.principal, "agent", j.agent, "expires_at", expiresAt)
	// Revoke before notifying the client so the credential is already
	// dead by the time the terminal event is observed (#157).
	j.revokeAll()
	j.emitTerminalError(arcp.CodeLeaseExpired, "lease expired during execution")
	j.cancel()
}

func (j *Job) timeout() {
	if !j.markTerminal(messages.StatusTimedOut) {
		return
	}
	j.revokeAll()
	j.emitTerminalError(arcp.CodeTimeout, "job exceeded max_runtime_sec")
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
	j.recordSeq(env.EventSeq)
	j.session.send(env)
	// Terminal fanout must not be gated by the job's own context, which
	// is typically cancelled at (or just before) the terminal event, or
	// subscribers could miss it (#82).
	j.session.srv.fanoutEvent(context.Background(), j.id, env)
}

// run executes the agent body inside the job's context.
func (j *Job) run() {
	// Reclaim the job from the registry on completion so the jobs/subs
	// maps do not grow unboundedly with terminal jobs (#81).
	defer j.session.srv.unregisterJob(j.id)
	if j.expiryTimer != nil {
		defer j.expiryTimer.Stop()
	}
	// Stop the max-runtime timer on exit so a completed job does not
	// leave the timer armed until its duration elapses (#80).
	if j.runtimeTimer != nil {
		defer j.runtimeTimer.Stop()
	}
	// Bail before reporting Running if the job is already terminal (an
	// already-expired lease at construction time) (#79).
	j.mu.Lock()
	term := j.terminal
	j.mu.Unlock()
	if term {
		return
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
				// Keep the goroutine dump and server-side file paths
				// server-local; the wire-facing error is generic so we
				// do not leak internal runtime details to clients.
				j.session.srv.opts.Logger.Error("agent panicked",
					"job_id", j.id,
					"agent", j.agent,
					"panic", fmt.Sprintf("%v", rec),
					"stack", string(debug.Stack()),
				)
				err = arcp.ErrInternalError.WithMessage("agent panicked")
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
			// Revoke before notifying the client so the credential is
			// already dead by the time the terminal event is observed (#157).
			j.revokeAll()
			j.emitTerminal(messages.TypeJobError, &body)
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
			j.revokeAll()
			j.emitTerminal(messages.TypeJobError, &body)
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
		j.revokeAll()
		j.emitTerminal(messages.TypeJobResult, &final)
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
			j.revokeAll()
			j.emitTerminal(messages.TypeJobError, &ebody)
			return
		}
		body.Output = raw
	}
	j.revokeAll()
	j.emitTerminal(messages.TypeJobResult, &body)
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
	j.retryRevoke(id, func(ctx context.Context) error {
		return j.session.srv.opts.Provisioner.Revoke(ctx, id)
	})
}

// revokePriorCredentialValue revokes only the prior value of a rotated
// credential (§9.8.2), keeping the credential id live with its new
// value. If the provisioner does not implement PriorValueRevoker it is
// assumed to own prior-value revocation itself, and nothing is revoked
// here (calling the id-keyed Revoke would kill the rotated-in value).
func (j *Job) revokePriorCredentialValue(id, priorValue string) {
	pv, ok := j.session.srv.opts.Provisioner.(credentials.PriorValueRevoker)
	if !ok {
		return
	}
	j.retryRevoke(id, func(ctx context.Context) error {
		return pv.RevokePriorValue(ctx, id, priorValue)
	})
}

// retryRevoke runs fn with bounded retries, using the runtime Clock for
// backoff so tests with a mock clock are deterministic (#91).
func (j *Job) retryRevoke(id string, fn func(ctx context.Context) error) {
	backoff := []time.Duration{50 * time.Millisecond, 250 * time.Millisecond, time.Second}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var err error
	for attempt := 0; attempt < len(backoff)+1; attempt++ {
		err = fn(ctx)
		if err == nil {
			return
		}
		if attempt == len(backoff) {
			break
		}
		done := make(chan struct{})
		timer := j.session.srv.opts.Clock.AfterFunc(backoff[attempt], func() { close(done) })
		select {
		case <-done:
		case <-ctx.Done():
			timer.Stop()
		}
		// Exit the whole retry loop (not just the select) once the
		// context is done, instead of calling Revoke on a dead ctx (#68).
		if ctx.Err() != nil {
			break
		}
	}
	j.session.srv.opts.Logger.Error("credential revocation failed", "job_id", j.id, "credential_id", id, "err", err)
}
