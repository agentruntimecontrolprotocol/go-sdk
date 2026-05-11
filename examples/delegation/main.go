// Fan a request out to peer runtimes; tolerate partial failure.
//
// The JobMux pattern is the Go-idiomatic answer to the Python
// async-iterator-fanout problem: a single goroutine reads
// client.Events() and routes by job_id to per-job channels.
package main

import (
	"context"
	"fmt"
	"sync"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
	"github.com/agentruntimecontrolprotocol/go-sdk/messages"
)

var peers = []string{"research.web", "research.code", "research.docs"}

func isTerminal(t string) bool {
	return t == "job.completed" || t == "job.failed" || t == "job.cancelled"
}

type Job struct {
	Target string
	JobID  arcp.JobID
	Final  *messages.JobCompleted
	Error  *messages.ErrorPayload
}

func delegate(ctx context.Context, c *Session,
	target, task string, traceID arcp.TraceID,
) Job {
	resp, err := c.Request(ctx, &arcp.Envelope{
		TraceID: traceID,
		Payload: &messages.AgentDelegate{
			Target: target,
			Task:   task,
			// trace_id propagates so peers join one distributed trace.
			Context: map[string]any{"trace_id": traceID},
		},
	})
	if err != nil {
		return Job{Target: target, Error: &messages.ErrorPayload{
			Code: arcp.CodeUnavailable, Message: err.Error()}}
	}
	acc, ok := resp.Payload.(*messages.JobAccepted)
	if !ok {
		return Job{Target: target, Error: &messages.ErrorPayload{
			Code: arcp.CodeFailedPrecondition, Message: "unexpected " + resp.Type()}}
	}
	return Job{Target: target, JobID: acc.JobID}
}

// JobMux: single reader on c.Events(), fanned out by job_id. Without
// this, parallel goroutines reading c.Events() starve each other.
type JobMux struct {
	c       *Session
	mu      sync.Mutex
	queues  map[arcp.JobID]chan arcp.Envelope
	startCh chan struct{}
}

func NewJobMux(c *Session) *JobMux {
	return &JobMux{c: c, queues: map[arcp.JobID]chan arcp.Envelope{}}
}

func (m *JobMux) Start(ctx context.Context) {
	go func() {
		for env := range m.c.Events(ctx) {
			if env.JobID == "" {
				continue
			}
			m.mu.Lock()
			ch, ok := m.queues[env.JobID]
			m.mu.Unlock()
			if !ok {
				continue
			}
			ch <- env
			if isTerminal(env.Type()) {
				close(ch)
			}
		}
	}()
}

func (m *JobMux) Register(jid arcp.JobID) <-chan arcp.Envelope {
	ch := make(chan arcp.Envelope, 16)
	m.mu.Lock()
	m.queues[jid] = ch
	m.mu.Unlock()
	return ch
}

func collect(mux *JobMux, j Job) Job {
	if j.Error != nil {
		return j
	}
	for env := range mux.Register(j.JobID) {
		switch p := env.Payload.(type) {
		case *messages.JobCompleted:
			j.Final = p
		case *messages.JobFailed:
			j.Error = &p.ErrorPayload
		case *messages.JobCancelled:
			j.Error = &messages.ErrorPayload{
				Code: arcp.CodeCancelled, Message: p.Reason}
		}
	}
	return j
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := openCoordinator(ctx) // transport, identity, auth elided
	defer c.Close(ctx)

	mux := NewJobMux(c)
	mux.Start(ctx)

	traceID := arcp.NewTraceID()
	request := "what changed in our auth stack in the last 30 days?"

	jobs := make([]Job, 0, len(peers))
	for _, peer := range peers {
		jobs = append(jobs, delegate(ctx, c, peer, request, traceID))
	}

	var wg sync.WaitGroup
	completed := make([]Job, len(jobs))
	for i, j := range jobs {
		i, j := i, j
		wg.Add(1)
		go func() { defer wg.Done(); completed[i] = collect(mux, j) }()
	}
	wg.Wait()

	fmt.Println(synthesize(request, completed))
}
