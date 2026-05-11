# cancellation

Two scenarios that exercise the §10.4–§10.5 control surface that
distinguishes ARCP from "agent over plain HTTP":

- `cancel`: cooperative termination with a deadline.
- `interrupt`: pause the job and route through a human, no termination.

## Before ARCP

Cancellation usually means closing the socket or trying to kill the
process. The agent's tool was already mid-network call, so it
either completes anyway (silent waste of money) or leaves a
half-applied side effect. There's no notion of "stop and ask"; the
only knob is "stop".

## With ARCP

```go
// Stop the job; runtime drives it to a clean checkpoint inside
// `deadline_ms` before terminating.
ack, _ := cancelJob(ctx, c, jid, "user_aborted",
    int(cancelDeadline/time.Millisecond))    // cancel.accepted
terminal, _ := awaitTerminal(ctx, c, jid)    // job.cancelled

// Or: pause the job, ask the human, resume.
interruptJob(ctx, c, jid,
    "Pause and ask before touching prod.")
// runtime emits human.input.request; answer with the HITL relay.
```

## ARCP primitives

- `cancel` cooperative contract — RFC §10.4 (`cancel.accepted` /
  `cancel.refused`, `deadline_ms`, escalation to `ABORTED`).
- `interrupt` (distinct from cancel) — §10.5; emits
  `human.input.request`, leaves the job in `blocked`.
- `capabilities.interrupt: false` fallback to `cancel` (advertised
  per §10.5; clients that find `interrupt: false` on a peer fall
  through to `cancel`).

## File tour

- `main.go` — two scenarios driven by `os.Args[1]`.
- `stubs.go` — `Session` shim.

## Variations

- Pair `interrupt` with [human_input](../human_input) for a working
  pause-and-ask loop.
- Send `cancel` against a `stream_id` instead of a `job_id` to
  terminate just one stream — terminal is a `stream.error` with
  `code: CANCELLED` (§10.4).
- Race many peers via [delegation](../delegation), cancel the
  slowest once N succeed.
