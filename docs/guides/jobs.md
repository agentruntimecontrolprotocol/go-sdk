# Jobs (§7)

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="../diagrams/job-lifecycle-dark.svg">
  <img alt="ARCP job lifecycle" src="../diagrams/job-lifecycle-light.svg">
</picture>

Submit work through the client:

```go
expires := time.Now().Add(10 * time.Minute)
h, err := cli.Submit(ctx, client.SubmitRequest{
    Agent:            "researcher@1.2.0",
    Input:            map[string]string{"topic": "leases"},
    LeaseRequest:     arcp.Lease{arcp.CapToolCall: {"search.*"}},
    LeaseConstraints: &messages.LeaseConstraints{ExpiresAt: &expires},
    IdempotencyKey:   "research-2026-W19",
    MaxRuntimeSec:    300,
    TraceID:          "" /* opt: 32-hex W3C trace id */,
})
```

`SubmitRequest` carries: the agent reference (bare `name`, or
`name@version`); the input payload; an optional `LeaseRequest`;
optional `LeaseConstraints` (currently `ExpiresAt`); an optional
`IdempotencyKey` (returns `DUPLICATE_KEY` against a 24-hour-scoped
key store on collision per principal); `MaxRuntimeSec` (returns
`TIMEOUT` with `final_status="timed_out"` when exceeded); and an
optional `TraceID`.

The runtime resolves the agent (`name@version` exact, `name` →
configured default, `name` → bare handler, `name` → lowest registered
version), creates a job, sends `job.accepted`, then invokes the
registered `server.AgentFunc`.

```go
srv.RegisterAgent("researcher", func(ctx context.Context, input json.RawMessage, jc *server.JobContext) (any, error) {
    return map[string]bool{"ok": true}, nil
})
srv.RegisterAgentVersion("researcher", "1.2.0", researcherV2)
_ = srv.SetDefaultAgentVersion("researcher", "1.2.0")
```

Returning a value emits `job.result`. Returning an error emits
`job.error` with the ARCP code from `arcp.Code(err)` and
`Retryable` from `arcp.IsRetryable(err)`.

## The `JobHandle`

`Submit` returns a `*client.JobHandle` exposing:

| Accessor | Use |
| --- | --- |
| `h.ID()` | The job id assigned by the runtime. |
| `h.Agent()` | The resolved `name@version`. |
| `h.Accepted()` | The full `messages.JobAccepted` payload (effective lease, budget, credentials). |
| `h.Events()` | Channel of inline `messages.JobEvent` (closed on terminal). |
| `h.Chunks()` | Channel of `messages.ResultChunkBody` for streamed results. |
| `h.CollectChunks(ctx)` | Concatenates all chunks into a single `[]byte`. |
| `h.Wait(ctx)` | Blocks until terminal; returns `(*JobResult, error)`. |
| `h.Result()`, `h.Err()`, `h.Done()` | Non-blocking terminal accessors. |
| `h.Cancel(ctx, reason)` | Sends `job.cancel`. |

## Cancellation

Only the submitting session can cancel:

```go
err := h.Cancel(ctx, "user requested stop")
```

Agents should watch `ctx.Done()` and return promptly. When
cancellation wins the terminal race, the runtime emits `job.error`
with `final_status: "cancelled"` and `code: CANCELLED`.

## Budgets

`cost.budget` entries live in the lease request:

```go
LeaseRequest: arcp.Lease{
    arcp.CapToolCall:   {"search.*"},
    arcp.CapCostBudget: {"USD:1.00"},
}
```

Agents emit cost metrics through `JobContext.Metric`; the runtime
debits matching budget currencies on every metric whose name starts
with `cost.`. When a counter drops to zero `JobContext.ValidateLeaseOp`
returns `BUDGET_EXHAUSTED` and the agent should propagate that error
out. See [Leases](./leases.md) for the full enforcement rules.

## Subscribing from another session

Read-only attach to a running job uses `Client.Subscribe`. The
subscription only succeeds when the subscriber's principal matches the
submitter's; otherwise the runtime returns `PERMISSION_DENIED`. The
subscriber observes the live event stream and (when
`SubscribeOptions.History` is true) any buffered history; it does not
inherit cancel authority. The subscription handle exposes the
`job.subscribed` payload via `sub.CurrentStatus()`, `sub.Agent()`,
`sub.Lease()`, `sub.ParentJobID()`, `sub.TraceID()`,
`sub.SubscribedFrom()`, and `sub.Replayed()`.
