# Package `client`

Import path: `github.com/agentruntimecontrolprotocol/go-sdk/client`

`client` owns the caller side of a session: handshake, feature
negotiation, job submission, cancellation, subscriptions, list jobs,
ack, resume, and result chunk collection.

## Entry point

```go
cli, err := client.Connect(ctx, t, client.Options{Token: token})
```

`Connect` performs the `session.hello` / `session.welcome` handshake
and returns a `*Client`. The transport `t` must already be open; the
client does not dial.

## Types

| Type | Purpose |
| --- | --- |
| `Client` | Connected ARCP session. |
| `Options` | Identity, token, advertised features, logger, auto-ack tuning, optional `Resume` block. |
| `SubmitRequest` | Agent, input, lease request, lease constraints, idempotency key, max runtime, trace id. |
| `JobHandle` | Accepted job state, events, chunks, cancel, wait. |
| `Subscription` | Live observer for an existing job (read-only). |
| `SubscribeOptions` | `FromEventSeq`, `History`. |
| `ListJobsRequest`, `JobList` | `session.list_jobs` shape. |

## `Client` surface

| Method | Purpose |
| --- | --- |
| `SessionID()` | Negotiated session id. |
| `Welcome()` | Full `*messages.SessionWelcome`. |
| `Features()` | Effective negotiated feature set. |
| `HasFeature(name)` | Predicate. |
| `HighestSeq()` | Largest `event_seq` seen — feeds `Resume.LastEventSeq`. |
| `Ack(ctx, seq)` | Manual `session.ack`. |
| `Submit(ctx, req)` | Submit a job; returns `*JobHandle`. |
| `Subscribe(ctx, jobID, opts)` | Attach read-only to a job; returns `*Subscription`. |
| `ListJobs(ctx, req)` | `session.list_jobs`; returns `*JobList`. |
| `Close(ctx)` | Send `session.bye` and close the transport. |

## `JobHandle` surface

| Method | Purpose |
| --- | --- |
| `ID()`, `Agent()` | Identity. |
| `Accepted()` | `*messages.JobAccepted` (effective lease, budget, credentials). |
| `Events()` | Channel of every job event; closed on terminal. |
| `Chunks()` | `result_chunk`-only channel; closed on terminal. |
| `CollectChunks(ctx)` | Reassemble a streamed result into `[]byte`. |
| `Wait(ctx)` | Block until terminal; returns `(*JobResult, error)`. |
| `Result()`, `Err()`, `Done()` | Non-blocking terminal accessors. |
| `Cancel(ctx, reason)` | Send `job.cancel`. |

## `Subscription` surface

| Method | Purpose |
| --- | --- |
| `JobID()`, `Agent()`, `CurrentStatus()`, `Lease()`, `ParentJobID()`, `TraceID()`, `SubscribedFrom()`, `Replayed()` | Fields captured from `job.subscribed`. |
| `Events()`, `Done()`, `Err()` | Live stream + terminal state. |
| `Close(ctx)` | Send `job.unsubscribe`. |

## Auto-ack

When `Options.AutoAckWindow > 0` and the `ack` feature is negotiated,
the client coalesces `session.ack` emission to one per
`AutoAckWindow` events processed, bounded by `AutoAckInterval`
(default `250ms`).

```go
cli, err := client.Connect(ctx, t, client.Options{Token: token})
h, err := cli.Submit(ctx, client.SubmitRequest{Agent: "echo", Input: input})
res, err := h.Wait(ctx)
```
