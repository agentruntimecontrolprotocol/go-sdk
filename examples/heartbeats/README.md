# heartbeats

A supervisor and a federated worker pool. Workers emit
`job.heartbeat` on a fixed cadence. If the supervisor misses two in
a row (RFC §10.3 default `N=2`), it re-dispatches the task to a
different worker — using the *same* `idempotency_key` so the
original worker, if it survived a transient blip, won't double-execute.

## Before ARCP

The supervisor pings `/healthz` every 5s and stops counting on
errors. There's no per-job liveness, so a worker stuck on a single
slow task looks healthy at the process level. Re-dispatch loses
state and risks duplicate side effects.

## With ARCP

```go
// supervisor side: drain heartbeats, reap stale workers, re-dispatch
case *messages.JobHeartbeat:
    w.LastHeartbeat = time.Now()
// reaper:
if now.Sub(w.LastHeartbeat) > deadline {
    dispatch(ctx, c, jobs[w.InFlight], roster, jobs)
}

// worker side: emit on a ticker until done
&messages.JobHeartbeat{Sequence: seq, DeadlineMilliseconds: ..., State: "running"}
```

Re-dispatch carries the same `idempotency_key`; the runtime
guarantees at-most-once effect (§6.4).

## ARCP primitives

- `job.heartbeat` cadence + `deadline_ms` — RFC §10.3.
- `idempotency_key` for safe re-dispatch — §6.4.
- `agent.delegate` for supervisor → worker fan-out — §14.
- Capability-advertised role on the worker session.

## File tour

- `main.go` — supervisor `dispatch()` + reaper, worker `execute()`
  + heartbeat loop.
- `work.go` — `doWork()` stub + `Session` shim.

## Variations

- Replace the in-memory roster with Redis sets keyed by role for
  multi-supervisor deploys.
- Pair with [resumability](../resumability) so a re-dispatched job
  resumes from the latest checkpoint instead of restarting.
- Promote the `(role)` worker tag into a runtime-side capability
  filter so `agent.delegate` selects role-eligible workers
  natively.
