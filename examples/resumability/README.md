# resumability

A durable research workflow that *actually crashes* mid-flight via
`os.Exit(137)` and resumes from the last checkpoint on the next
process boot. The runtime kept every envelope and every per-step
idempotency receipt — replay yields the same outputs without
re-running the LLM.

## Before ARCP

The workflow lives in a process. The process dies, the work is
gone. Frameworks that promise "durable workflows" usually require
you to write all step IO through their proprietary API; the agent
loses access to streaming partial results in exchange.

## With ARCP

```sh
# crash run
CRASH_AFTER_STEP=synthesize go run ./examples/resumability
# → prints RESUME_JOB_ID=... RESUME_CHECKPOINT_ID=...

# resume run
RESUME_JOB_ID=... RESUME_AFTER_MSG_ID=... RESUME_CHECKPOINT_ID=... \
    go run ./examples/resumability
```

```go
key := stepKey(jid, step, fmt.Sprintf("%v", output))   // RFC §6.4
output, _ = runStep(ctx, c, jid, step, map[string]any{
    "prior":           output,
    "idempotency_key": key,                            // dedupe on resume
})
emitCheckpoint(ctx, c, jid, step)                      // RFC §10.1
```

## ARCP primitives

- `workflow.start` / `job.checkpoint` / `job.completed` — RFC §10.1.
- `resume` envelope with `after_message_id` + `checkpoint_id` — §19.
- `idempotency_key` per step — §6.4.
- `subscription.backfill_complete` synthetic marker — §13.3.
- `DATA_LOSS` if retention has expired — §18.2.

## File tour

- `main.go` — `executeSteps()` + `issueResume()` driver.
- `steps.go` — `runStep()` stub + `Session` shim.

## Variations

- Swap the in-process step bodies for [delegation](../delegation)'d
  peer jobs; resume becomes a fan-out replay.
- Add `job.heartbeat` (see [heartbeats](../heartbeats)) so the
  supervisor can re-dispatch instead of waiting for the user to
  re-trigger resume.
- Keep the workflow in `paused` instead of crashing: emit
  `interrupt` (see [cancellation](../cancellation)) and resume
  after a human OK.
