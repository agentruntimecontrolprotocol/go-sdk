// Supervisor + worker pool. Heartbeat loss reroutes via idempotency_key.
package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
	"github.com/agentruntimecontrolprotocol/go-sdk/messages"
)

const (
	heartbeatInterval = 15 * time.Second
	deadline          = heartbeatInterval * 2 // RFC §10.3 default N=2
)

type Worker struct {
	ID            string
	Role          string
	LastHeartbeat time.Time
	InFlight      arcp.JobID
}

type Task struct {
	ID, Role, IdempotencyKey string
	Payload                  map[string]any
}

type Roster struct {
	mu      sync.Mutex
	workers map[string]*Worker
}

func (r *Roster) idle(role string) *Worker {
	r.mu.Lock()
	defer r.mu.Unlock()
	var pick *Worker
	for _, w := range r.workers {
		if w.Role != role || w.InFlight != "" {
			continue
		}
		if pick == nil || w.LastHeartbeat.Before(pick.LastHeartbeat) {
			pick = w
		}
	}
	return pick
}

// Supervisor side --------------------------------------------------------

func dispatch(ctx context.Context, c *Session, task Task,
	roster *Roster, jobs map[arcp.JobID]Task,
) error {
	worker := roster.idle(task.Role)
	if worker == nil {
		return fmt.Errorf("no idle workers for role=%s", task.Role)
	}
	// Same idempotency_key on every re-dispatch (RFC §6.4): a worker
	// that survived the network blip dedupes; it doesn't re-execute.
	resp, err := c.Request(ctx, &arcp.Envelope{
		IdempotencyKey: task.IdempotencyKey,
		Payload: &messages.AgentDelegate{
			Target:  worker.ID,
			Task:    task.ID,
			Context: map[string]any{"task_payload": task.Payload},
		}})
	if err != nil {
		return err
	}
	jid := resp.Payload.(*messages.JobAccepted).JobID
	worker.InFlight = jid
	jobs[jid] = task
	return nil
}

func reap(ctx context.Context, c *Session, roster *Roster, jobs map[arcp.JobID]Task) {
	t := time.NewTicker(heartbeatInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			now := time.Now()
			for _, w := range roster.workers {
				if now.Sub(w.LastHeartbeat) <= deadline {
					continue
				}
				if task, ok := jobs[w.InFlight]; ok {
					delete(jobs, w.InFlight)
					_ = dispatch(ctx, c, task, roster, jobs) // re-dispatch
				}
				delete(roster.workers, w.ID)
			}
		}
	}
}

func supervise(ctx context.Context, c *Session, roster *Roster, jobs map[arcp.JobID]Task) {
	go reap(ctx, c, roster, jobs)
	for env := range c.Events(ctx) {
		switch env.Payload.(type) {
		case *messages.JobHeartbeat:
			for _, w := range roster.workers {
				if w.InFlight == env.JobID {
					w.LastHeartbeat = time.Now()
				}
			}
		case *messages.JobCompleted, *messages.JobFailed, *messages.JobCancelled:
			delete(jobs, env.JobID)
			for _, w := range roster.workers {
				if w.InFlight == env.JobID {
					w.InFlight = ""
				}
			}
		}
	}
}

// Worker side ------------------------------------------------------------

func heartbeatLoop(ctx context.Context, c *Session, jid arcp.JobID, stop <-chan struct{}) {
	t := time.NewTicker(heartbeatInterval)
	defer t.Stop()
	for seq := 0; ; seq++ {
		_ = c.Send(ctx, &arcp.Envelope{
			JobID: jid,
			Payload: &messages.JobHeartbeat{
				Sequence: seq, State: messages.JobStateRunning,
				DeadlineMilliseconds: int(deadline / time.Millisecond),
			}})
		select {
		case <-stop:
			return
		case <-t.C:
		}
	}
}

func execute(ctx context.Context, c *Session, env arcp.Envelope) {
	jid := arcp.NewJobID()
	_ = c.Send(ctx, &arcp.Envelope{JobID: jid, CorrelationID: env.ID,
		Payload: &messages.JobAccepted{JobID: jid}})
	_ = c.Send(ctx, &arcp.Envelope{JobID: jid,
		Payload: &messages.JobStarted{StartedAt: time.Now()}})

	stop := make(chan struct{})
	go heartbeatLoop(ctx, c, jid, stop)
	defer close(stop)

	d := env.Payload.(*messages.AgentDelegate)
	payload, _ := d.Context["task_payload"].(map[string]any)
	result, err := doWork(ctx, payload)
	if err != nil {
		_ = c.Send(ctx, &arcp.Envelope{JobID: jid,
			Payload: &messages.JobFailed{ErrorPayload: messages.ErrorPayload{
				Code: arcp.CodeInternal, Message: err.Error(), Retryable: true,
			}}})
		return
	}
	_ = c.Send(ctx, &arcp.Envelope{JobID: jid, Payload: marshalCompleted(result)})
}

func runWorker(ctx context.Context, c *Session) {
	for env := range c.Events(ctx) {
		if _, ok := env.Payload.(*messages.AgentDelegate); ok {
			go execute(ctx, c, env)
		}
	}
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	supervisor := openSupervisor(ctx) // identity (privileged), auth elided
	defer supervisor.Close(ctx)
	roster := &Roster{workers: map[string]*Worker{}}
	jobs := map[arcp.JobID]Task{}

	// In production each worker is its own process; co-hosted here for the demo.
	for _, role := range []string{"indexer", "extractor", "archiver"} {
		for i := 0; i < 2; i++ {
			w := openWorker(ctx) // worker session, capabilities advertise role
			go runWorker(ctx, w)
			id := fmt.Sprintf("%s-%d", role, i)
			roster.workers[id] = &Worker{
				ID: id, Role: role, LastHeartbeat: time.Now(),
			}
		}
	}
	go supervise(ctx, supervisor, roster, jobs)

	roles := []string{"indexer", "extractor", "archiver"}
	for n := 0; n < 6; n++ {
		if err := dispatch(ctx, supervisor, Task{
			ID: fmt.Sprintf("t%03d", n), Role: roles[n%3],
			Payload:        map[string]any{"shard": n},
			IdempotencyKey: fmt.Sprintf("openclaw:t%03d", n),
		}, roster, jobs); err != nil {
			log.Print(err)
		}
	}
	time.Sleep(60 * time.Second)
}
