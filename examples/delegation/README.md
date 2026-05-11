# delegation

Fan one user request out to N peer runtimes via `agent.delegate`,
collect the results back, synthesize. The interesting bit in Go is
the `JobMux`: a single goroutine reads `c.Events()` and routes by
`job_id` to per-job channels.

## Before ARCP

The orchestrator spawns N HTTP requests (or N RPCs over a custom
fan-out), waits on a wait group, and merges. Failure modes —
partial success, peer crash, peer slow — each get an ad-hoc retry
policy that doesn't talk to the others.

## With ARCP

```go
mux := NewJobMux(c)
mux.Start(ctx)
for _, peer := range peers {
    jobs = append(jobs, delegate(ctx, c, peer, request, traceID))
}
for _, j := range jobs {
    go func(j Job) { results <- collect(mux, j) }(j)
}
```

`collect()` walks the per-job channel until terminal; `synthesize()`
gets a slice of `Job` with either `Final` or `Error` populated.

## ARCP primitives

- `agent.delegate` fan-out — RFC §14.
- `trace_id` propagated to peers for one distributed trace — §17.3.
- `idempotency_key` would let the orchestrator retry a peer without
  duplicate execution (§6.4).

## File tour

- `main.go` — `delegate()` per peer + `JobMux` + `collect()` aggregator.
- `synth.go` — final synthesizer stub + `Session` shim.

## Variations

- Race-and-cancel: as soon as N succeed, send `cancel` to the laggers
  (see [cancellation](../cancellation)).
- Use [capability_negotiation](../capability_negotiation) to pick
  the per-peer subset by cost / latency.
- Mirror the trace into [subscriptions](../subscriptions) for live
  fan-out visualization.
