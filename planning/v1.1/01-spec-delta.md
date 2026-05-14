# 01 — Spec Delta: ARCP v1.0 → v1.1

Source: `../spec/docs/draft-arcp-02.1.md`. v1.1 is additive; the
envelope (`arcp:"1"`, `id`, `type`, `session_id`, `trace_id?`,
`job_id?`, `event_seq?`, `payload`) is unchanged and a v1.0 client
sees a v1.1 runtime as a v1.0 runtime that happens to ignore unknown
fields and advertises extra features it never asks the client to use.

## Additions table

Status legend: **A** additive (no v1.0 client impact), **N** new
envelope/feature that v1.0 clients ignore by §5.1 forward-compat,
**Need** what a v1.1 Go client/runtime must do.

| §       | Feature                              | MUST/SHOULD/MAY | Type | Need (Go side)                                                                                                                            |
| ------- | ------------------------------------ | --------------- | ---- | ----------------------------------------------------------------------------------------------------------------------------------------- |
| 6.2     | `capabilities.features: []string`    | MUST advertise  | A    | Add `Features []string` to hello/welcome capabilities; build intersection at handshake; expose `Client.HasFeature` / `Session.HasFeature` |
| 6.2     | `agents` rich shape `{name,versions,default}` | MUST when `agent_versions` negotiated | A | `AgentInventory` struct; client accepts union of `[]string` (v1.0 shape) and `[]AgentEntry` (v1.1)                                  |
| 6.4     | `session.ping` / `session.pong`      | SHOULD honour   | N    | New envelope types; ticker per peer; not counted in `event_seq`; runtime sets `heartbeat_interval_sec` on welcome                         |
| 6.4     | `HEARTBEAT_LOST` after 2 silent intervals | MAY            | N    | Watchdog on both peers; transport close; runtime keeps jobs alive for resume window                                                       |
| 6.5     | `session.ack { last_processed_seq }` | MAY             | N    | Server records per-session highwater; eligible to free buffer ≤ ack; client `AutoAck` coalescer; not counted in `event_seq`               |
| 6.5     | `status { phase:"back_pressure" }`   | MAY             | A    | Lag threshold (configurable); emitted as a normal `job.event`                                                                             |
| 6.6     | `session.list_jobs` / `session.jobs` | MAY             | N    | Filter `{status?, agent?, created_after?, created_before?}`, `limit`, opaque `cursor`; same-principal auth default                        |
| 7.1     | `lease_constraints.expires_at`       | OPTIONAL        | A    | Submit + echo on accepted; validate ISO 8601 UTC + future; clock injection on validateLeaseOp                                             |
| 7.1     | `budget` initial counters on accepted | MUST when lease has `cost.budget` | A | `map[currency]float64` per job; lock to lease-grant moment                                                                          |
| 7.5     | `agent ::= name "@" version`         | MAY             | A    | Parser/formatter; resolve bare to default; running job's version is fixed                                                                  |
| 7.5     | Default-version resolution           | MAY             | A    | Inventory carries `default`; bare names resolve through it; pinned names require exact match                                              |
| 7.6     | `job.subscribe` / `job.subscribed` / `job.unsubscribe` | MAY | N | Re-attach from a different session; replay buffered when `history:true`; subscriber-scoped event_seq on replay                  |
| 7.6     | Subscribers MUST NOT cancel          | MUST            | A    | `handleJobCancel` returns `PERMISSION_DENIED` for non-submitter sessions                                                                  |
| 8.2.1   | `progress { current, total?, units?, message? }` | MAY  | A    | New reserved kind; non-negative `current`; recommended `current ≤ total`                                                                  |
| 8.4     | `result_chunk { result_id, chunk_seq, data, encoding, more }` | MAY | A | Streaming writer; terminating `job.result` carries `result_id` + `result_size`; MUST NOT mix inline result with chunks            |
| 9.4     | Child `cost.budget` ≤ parent's REMAINING per currency | MUST | A | Delegation subset check consults live counters, not lease grant                                                                       |
| 9.4     | Child `expires_at` ≤ parent's        | MUST            | A    | If parent has `expires_at` and child omits, child inherits implicitly                                                                     |
| 9.5     | `LEASE_EXPIRED` enforcement          | MUST            | A    | `validateLeaseOp` takes `now` and `constraints`; runtime watchdog terminates with `job.error{LEASE_EXPIRED}`                              |
| 9.6     | `cost.budget` lease capability       | OPTIONAL        | A    | `currency:decimal` grammar; counters init from lease; debit on `metric` events with `name` starting `cost.` and matching `unit`           |
| 9.6     | Budget enforcement before any leased op | MUST when budget present | A | If any counter ≤ 0 → `BUDGET_EXHAUSTED` (surface preferred as `tool_result` error)                                                |
| 9.6     | `cost.budget.remaining` metric       | MAY             | A    | Debounced emit (TS uses 5% threshold); read snapshot via `JobContext.Budget()`                                                            |
| 11      | OTel attrs `arcp.lease.expires_at`, `arcp.budget.remaining` | SHOULD | A | Added by OTel adapter when capabilities present                                                                                          |
| 12      | `LEASE_EXPIRED`, `BUDGET_EXHAUSTED`, `AGENT_VERSION_NOT_AVAILABLE` | MUST | A | All three non-retryable                                                                                                            |

Nothing in v1.1 is breaking. A v1.0-only Go client sending no
`features` array continues to function against a v1.1 runtime; the
runtime simply will not use any of the new features against it
(§6.2 closing paragraph).

## New error codes (§12)

Three additions to the v1.0 set of twelve, all `retryable: false`.

| Code                            | Raised at                                                                                            | Go-side surface                                                                |
| ------------------------------- | ---------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------ |
| `AGENT_VERSION_NOT_AVAILABLE`   | `job.submit` whose `agent@version` is not in the inventory (§7.5)                                    | `arcp.ErrAgentVersionNotAvailable` sentinel; runtime emits `session.error`     |
| `LEASE_EXPIRED`                 | (a) any `validateLeaseOp` past `expires_at`; (b) runtime watchdog firing while job runs (§9.5)       | `arcp.ErrLeaseExpired`; surfaces as `tool_result` body.error or `job.error`    |
| `BUDGET_EXHAUSTED`              | `validateLeaseOp` when any counter ≤ 0 (§9.6); preferred surface is `tool_result` so agent can react | `arcp.ErrBudgetExhausted`; check after each `cost.*` metric is applied         |

All three live in `errors.go` next to the v1.0 codes; the package
preserves the `LEASE_SUBSET_VIOLATION` raise-site in `lease.go` and
the `PERMISSION_DENIED` raise-site in `lease.go` and `server.go`.

`retryable: false` matters for the Go client: `arcp.IsRetryable(err)`
returns `false` against these three sentinels, so generic
exponential-backoff loops do not retry them.

## Capability negotiation table (§6.2)

Effective feature set is the intersection of both peers'
`capabilities.features` lists. Feature names are exact strings.

| Flag                | Section | What it gates on the wire                                          | Negotiation side                          |
| ------------------- | ------- | ------------------------------------------------------------------ | ----------------------------------------- |
| `heartbeat`         | §6.4    | `session.ping`, `session.pong`, `heartbeat_interval_sec` semantics | Either may initiate ping                  |
| `ack`               | §6.5    | `session.ack`; runtime back-pressure status                        | Client initiates                          |
| `list_jobs`         | §6.6    | `session.list_jobs`, `session.jobs`                                | Client initiates                          |
| `subscribe`         | §7.6    | `job.subscribe`, `job.subscribed`, `job.unsubscribe`               | Client initiates                          |
| `lease_expires_at`  | §9.5    | `lease_constraints.expires_at` field                               | Client requests via `job.submit`          |
| `cost.budget`       | §9.6    | `cost.budget` lease capability; counters; `BUDGET_EXHAUSTED`       | Client requests via `lease_request`       |
| `progress`          | §8.2.1  | `progress` event kind                                              | Runtime/agent emits                       |
| `result_chunk`      | §8.4    | `result_chunk` events + `job.result.result_id` form                | Runtime/agent emits                       |
| `agent_versions`    | §7.5    | `name@version` grammar; rich `agents` inventory shape              | Both: runtime publishes, client pins      |

Either peer using a feature outside the intersection MUST be rejected
locally. The Go SDK enforces this in two places: client helpers (e.g.
`Client.Ack`, `Client.ListJobs`, `Client.Subscribe`) return
`INVALID_REQUEST` if the feature was not negotiated; the runtime
handler tables refuse to dispatch envelopes whose feature is absent
(also `INVALID_REQUEST`).

The canonical Go feature list lives in
`internal/version/features.go` mirroring TS
`packages/core/src/version.ts:V1_1_FEATURES`. Helper:
`features.Intersect(a, b []string) []string`.

## What is _not_ in v1.1 (deferred)

Spec §"Not in v1.1": pause/unpause, priority/scheduling hints,
federation, streaming-token surface for LLM outputs. The Go SDK
MUST NOT ship any of these even informally; they are not protocol
errors today but are reserved.
