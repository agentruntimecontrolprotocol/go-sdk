# 02 — Current-State Audit (Go SDK)

## Headline finding

**This module does not implement ARCP v1.0.** It implements a
different in-repo design — see `RFC-0001-v2.md` in the module root
and the TODO at `messages/session.go:9` referencing "RFC §6.2". The
wire vocabulary, envelope shape, and error taxonomy are all
inconsistent with `../spec/docs/draft-arcp-02.md`. Setting `arcp:"1.0"`
in [version.go:5](../version.go) does not make the on-the-wire format
v1.0-compatible.

Concrete divergences:

| Concern               | v1.0/v1.1 spec                                                                                                    | Current go-sdk (`RFC-0001-v2`)                                                                                                                                              |
| --------------------- | ----------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Envelope `arcp`       | literal `"1"` (§5.1)                                                                                              | `"1.0"` ([version.go:5](../version.go))                                                                                                                                     |
| Envelope fields       | `arcp`, `id`, `type`, `session_id?`, `job_id?`, `trace_id?`, `event_seq?`, `payload`                              | adds `timestamp`, `source`, `target`, `stream_id`, `subscription_id`, `span_id`, `parent_span_id`, `correlation_id`, `causation_id`, `idempotency_key`, `priority`, `extensions` ([envelope.go:38](../envelope.go))      |
| Session handshake     | `session.hello` → `session.welcome` (or `session.error`); `session.bye` to close (§6.1–§6.7)                       | `session.open`/`session.challenge`/`session.authenticate`/`session.accepted`/`session.unauthenticated`/`session.rejected`/`session.refresh`/`session.evicted`/`session.close` ([messages/session.go](../messages/session.go))            |
| Job lifecycle         | `job.submit` → `job.accepted` → `job.event*` → `job.result` ∣ `job.error`; `job.cancel` (§7.1–§7.4)                | `job.accepted`/`job.started`/`job.progress`/`job.heartbeat`/`job.checkpoint`/`job.completed`/`job.failed`/`job.cancelled`/`job.schedule`/`tool.invoke`/`tool.result`/`tool.error`/`agent.delegate`/`agent.handoff` ([messages/execution.go](../messages/execution.go))      |
| Event channel         | one `job.event` envelope, `payload.kind` ∈ {log,thought,tool_call,tool_result,status,metric,artifact_ref,delegate,progress,result_chunk} (§8.2) | per-kind envelope types: `stream.open/chunk/close/error`, `subscribe.event`, `event.emit`, `log`, `metric`, `trace.span`, `permission.*`, `human.input.*`, `artifact.*` ([messages/streaming.go](../messages/streaming.go), [messages/subscriptions.go](../messages/subscriptions.go), [messages/telemetry.go](../messages/telemetry.go), [messages/permissions.go](../messages/permissions.go), [messages/human.go](../messages/human.go), [messages/artifacts.go](../messages/artifacts.go))    |
| Error codes           | 12 v1.0 + 3 v1.1 (§12)                                                                                            | 19 gRPC-style codes — `INVALID_ARGUMENT`, `DEADLINE_EXCEEDED`, `RESOURCE_EXHAUSTED`, `FAILED_PRECONDITION`, `ABORTED`, `OUT_OF_RANGE`, `UNIMPLEMENTED`, `BACKPRESSURE_OVERFLOW`, etc. ([errors.go:14-37](../errors.go)) |
| Cancellation          | `job.cancel { reason? }` from owning session only (§7.4)                                                          | generic `cancel` / `cancel.accepted` / `cancel.refused` / `interrupt` envelopes ([messages/control.go:13-17](../messages/control.go))                                       |
| Heartbeats            | `session.ping`/`session.pong`; session-scoped (§6.4)                                                              | `ping`/`pong` envelopes ([messages/control.go:9-10](../messages/control.go)) + `job.heartbeat` payload ([messages/execution.go:103](../messages/execution.go))              |
| Subscribe             | `job.subscribe`/`job.subscribed`/`job.unsubscribe` for cross-session attach (§7.6)                                | generic `subscribe`/`subscribe.accepted`/`subscribe.event`/`subscribe.closed`/`unsubscribe` over arbitrary filters ([messages/subscriptions.go](../messages/subscriptions.go)) — orthogonal concept            |
| Leases                | immutable per-job; capability → glob[]; reserved namespaces (§9)                                                  | `lease.granted`/`lease.extended`/`lease.revoked`/`lease.refresh` model with explicit revoke + refresh ([messages/permissions.go:14-17](../messages/permissions.go))         |
| Resume                | `session.hello.payload.resume` + `last_event_seq`; rotating `resume_token` (§6.3)                                 | `resume` envelope ([messages/control.go:17](../messages/control.go)); no rotating token primitive surfaced                                                                  |
| Transport             | WebSocket MUST; stdio MUST for in-process children (§4)                                                           | Only an in-process `memory` transport ([transport/memory.go](../transport/memory.go)); no WebSocket, no stdio                                                               |
| Storage               | event log with session-scoped monotonic `seq`; resume replay (§6.3, §8.3)                                         | SQLite event log via modernc.org/sqlite ([store/eventlog.go](../store/eventlog.go)); seq tracked but the message types it indexes are the wrong protocol                    |

**Conclusion.** "Migrating this SDK to v1.1" is not an additive
patch. The wire vocabulary and envelope schema must be replaced. The
question for planning is which existing scaffolding survives the
rewrite. The honest answer is: most of the code shape (Transport
interface, slog use, Verifier abstraction, ULID ids, SQLite event
log structure, examples directory pattern) survives; almost none of
the `messages/` package, the `runtime/` handshake, or the envelope
schema does.

## v1.0 conformance — section by section

Measured against `../typescript-sdk/CONFORMANCE.md` (which lists the
TS surface section-by-section) and `../spec/docs/draft-arcp-02.md`.

| Spec §              | TS status   | Go status                | Citation                                                                                                          |
| ------------------- | ----------- | ------------------------ | ----------------------------------------------------------------------------------------------------------------- |
| §4.1 WebSocket      | Implemented | **Missing**              | No `transport/websocket.go`; only in-memory ([transport/](../transport/))                                         |
| §4.2 stdio NDJSON   | Implemented | **Missing**              | No `transport/stdio.go`                                                                                           |
| §4.3 MemoryTransport| Implemented | **Present (structure)**  | [transport/memory.go](../transport/memory.go) — semantics are right; the envelopes it carries are wrong          |
| §5.1 envelope shape | Implemented | **Wrong shape**          | [envelope.go:38-82](../envelope.go) has ~13 extra fields not in §5.1                                              |
| §5.1 `arcp:"1"`     | Implemented | **Wrong value**          | [version.go:5](../version.go) sets `"1.0"`                                                                        |
| §5.1 unknown-field passthrough | Implemented | **Partial**       | [envelope.go:88-108](../envelope.go) envelopeWire mirrors known fields; `messages/` payloads use plain structs, will fail on unknown nested fields |
| §6.1 bearer auth    | Implemented | **Present, wrong shape** | [auth/bearer.go](../auth/bearer.go) exists, but consumes `messages.Auth.Token` keyed off the v1.1-ish `session.open` payload, not `session.hello.payload.auth` |
| §6.2 hello/welcome  | Implemented | **Missing**              | Different handshake entirely ([messages/session.go:10-20](../messages/session.go))                                |
| §6.3 resume         | Implemented | **Partial scaffolding**  | EventLog ([store/eventlog.go](../store/eventlog.go)) supports SELECT-by-seq but `resume_token` rotation is absent |
| §6.7 `session.bye`  | Implemented | **Missing**              | Closest equivalent is `session.close` ([messages/session.go:106](../messages/session.go))                         |
| §7.1 `job.submit`   | Implemented | **Missing**              | No submit verb; the runtime issues `job.accepted` proactively                                                     |
| §7.2 idempotency    | Implemented | **Partial**              | Envelope-level `idempotency_key` exists ([envelope.go:74](../envelope.go)); no `(principal, key)` dedupe map      |
| §7.3 states         | Implemented | **Different**            | Includes `blocked`, `paused`, `queued` ([messages/execution.go:37-45](../messages/execution.go)) — not v1.0       |
| §7.4 cancel         | Implemented | **Different**            | Generic `cancel` envelope, not `job.cancel`                                                                       |
| §8.1 `job.event`    | Implemented | **Missing**              | Events are emitted as discrete envelope types, not as a single `job.event` envelope with a `kind`                  |
| §8.2 8 reserved kinds | Implemented | **Missing**              | The eight kinds are split across `streaming.go`, `telemetry.go`, `permissions.go`, `human.go`, `artifacts.go`     |
| §8.3 session-scoped monotonic seq | Implemented | **Partial**    | [store/eventlog.go:36](../store/eventlog.go) keeps per-session seq counters; not wired into a "single counter across concurrent jobs" emission path |
| §9 leases           | Implemented | **Different model**      | "grant/extend/revoke/refresh" model ([messages/permissions.go](../messages/permissions.go)), not v1.0 immutable-per-job + glob                  |
| §10 delegation      | Implemented | **Missing**              | `agent.delegate` exists ([messages/execution.go:188](../messages/execution.go)) but as a "deferred to v0.2" stub  |
| §11 trace           | Implemented | **Partial**              | TraceID/SpanID on envelope ([envelope.go:60-66](../envelope.go)); no OTel emitter; `trace_id` is W3C 32-hex-string per §11 but envelope's `TraceID` type is not validated |
| §12 12 error codes  | Implemented | **0/12 match**            | Code names differ entirely ([errors.go:16-37](../errors.go))                                                      |
| §14 security        | Implemented | **N/A**                  | resume-window sweep, per-session DoS caps — no equivalent                                                         |
| §15 `x-vendor.*`    | Implemented | **Missing**              | `extensions` map exists on envelope ([envelope.go:78](../envelope.go)) but classifier/policy is absent            |

**Net.** Zero of the §4–§15 v1.0 normative requirements are
implemented to spec. Some have aspirational scaffolding (transport
interface, event log, slog) that survives; the protocol surface does
not.

## Package map

| Package                                                | Files | Purpose (as declared)                          | Distance from v1.0/v1.1 spec                                                                                          |
| ------------------------------------------------------ | ----- | ---------------------------------------------- | --------------------------------------------------------------------------------------------------------------------- |
| `github.com/agentruntimecontrolprotocol/go-sdk` (root) | 7     | envelope, ids, errors, extensions, trace       | Envelope shape and error codes diverge; ULID/UUIDv7 ids correct in principle; trace W3C-ish but not validated         |
| `auth/`                                                | 3     | Verifier interface, AnonymousVerifier, bearer  | Interface shape survives; consumes wrong `Auth` payload shape                                                          |
| `client/`                                              | 2     | `Client` wrapper around Transport              | Talks the wrong handshake; needs full rewrite to `session.hello`/`session.welcome` flow                                |
| `cmd/arcp/`                                            | 1 dir | CLI binary                                     | Empty subdirectory; matches TS `@arcp/sdk` CLI target                                                                  |
| `examples/`                                            | 14 dirs | runnable example programs                    | Subjects are right (heartbeats, subscriptions, leases, resumability, cancellation, delegation, capability negotiation) but they exercise the wrong protocol |
| `internal/deadline/`, `internal/ulid/`                 | —     | helpers                                        | Salvageable; the ULID helper duplicates `oklog/ulid/v2` use                                                            |
| `messages/`                                            | 12    | typed payloads per message group               | Whole-package rewrite. Every wire-type constant and every payload struct changes.                                      |
| `runtime/`                                             | 2     | `Runtime.Serve` / `handshake` / `runLoop`      | Whole-package rewrite of handshake; session/job FSM has the wrong states and verbs                                     |
| `store/`                                               | 4     | SQLite event log                               | Schema and seq logic survives; the envelopes it persists change. Keep `modernc.org/sqlite` for portability              |
| `tests/`                                               | 2     | cross-package integration                      | Currently asserts the wrong handshake; replace                                                                         |
| `transport/`                                           | 3     | Transport interface + memory                   | Interface survives. WebSocket and stdio implementations are new                                                        |

## Gap matrix — v1.1 features × current state

`M` missing, `P` partial scaffolding, `S` present-but-spec-incompatible.

| §       | Feature              | State | Target package         | Risk | Go-specific friction                                                                                                                   |
| ------- | -------------------- | ----- | ---------------------- | ---- | -------------------------------------------------------------------------------------------------------------------------------------- |
| 6.2     | `features[]` negotiation | M     | core (`messages/session.go`) | L    | —                                                                                                                                      |
| 6.2     | rich `agents` shape  | M     | core + server          | L    | Union types in Go: model as `AgentInventory` with `Names []string` and `Agents []AgentEntry`; one or the other is populated            |
| 6.4     | heartbeat ping/pong  | M     | server + client        | M    | Goroutine leak risk if read-loop returns before draining ping-channel; closing order must be: stop ticker → drain → return             |
| 6.4     | `HEARTBEAT_LOST` close | M    | server + client        | M    | `time.AfterFunc` resets on each inbound envelope; cancel both directions atomically when triggered                                     |
| 6.5     | `session.ack`        | M     | server + client        | M    | Per-session highwater under `sync.Mutex`; ack coalescer uses `time.NewTimer` + `Reset`, not `time.Sleep`; bounded `select`              |
| 6.5     | back-pressure status | M     | server                 | L    | Threshold check on every emit is cheap; gated behind `metricInterceptor` style hook                                                    |
| 6.6     | `list_jobs` / `jobs` | M     | server + client        | **H** | Cursor pagination must be re-entrant against `context.Cancellation`: opaque cursor = base64(JSON{after_id, ts}); never embed pointers; `ctx.Err()` checked between page reads |
| 7.5     | `name@version` grammar | M    | core                   | L    | `agent.ParseRef` returns `(name, version, error)`; format respects `[a-z0-9][a-z0-9._-]*` and `[a-zA-Z0-9.+_-]+`                       |
| 7.5     | inventory + default  | M     | server                 | M    | `Server.RegisterAgent(name, fn)` / `RegisterAgentVersion(name, version, fn)` / `SetDefaultAgentVersion`; concurrency-safe map writes only at start-up; reads lock-free after `Start` |
| 7.6     | `job.subscribe`      | M     | server + client        | **H** | Replay uses subscriber's own `event_seq` space — must allocate seq under the subscriber's session lock, not the owning session's. Fan-out via a per-subscriber `<-chan Event` with bounded buffer to prevent slow-subscriber back-pressure on owning session |
| 7.6     | cancel auth         | M     | server                 | L    | Centralize submitter check at `handleJobCancel` entry; PERMISSION_DENIED, not 403-style                                                |
| 8.2.1   | `progress` body      | M     | core                   | L    | `current` non-negative — enforce in `parseEventBody`                                                                                   |
| 8.4     | `result_chunk`       | M     | core + runtime         | M    | Return `<-chan ResultChunk` from `JobHandle.Chunks()`; `JobContext.StreamResult` is an `io.Writer` (or `WriteCloser`) wrapper that auto-numbers `chunk_seq`; MUST NOT mix inline result with chunks |
| 9.4     | budget subset        | M     | runtime/lease          | M    | Delegation subset reads live counter (`atomic.Int64`-backed map of micros), not lease grant; race window between read and child accept must be a single critical section |
| 9.5     | `lease_expires_at`   | M     | core + runtime/lease   | M    | Inject `now` into `ValidateLeaseOp` for testability; watchdog uses `time.AfterFunc(expires - now, …)` not `time.Sleep`                  |
| 9.6     | `cost.budget` counters | M    | runtime/lease + runtime/job | M | Per-currency counters under `sync.Mutex`; debit applies atomically with the check; preferred surface is `tool_result` body.error       |
| 9.6     | `cost.budget.remaining` metric | M | runtime              | L    | 5% debounce mirrors TS                                                                                                                  |
| 11      | OTel v1.1 attrs      | M     | middleware/otel        | L    | Only the api package (`go.opentelemetry.io/otel`); runtime is consumer's choice                                                         |
| 12      | 3 new error codes    | M     | errors.go              | L    | Sentinels mirror the v1.0 set                                                                                                          |

The high-risk items are §6.6 (`list_jobs` cursor) and §7.6 (subscribe
fan-out): both are the kinds of Go problems where naive
implementations leak goroutines and deadlock under cancel. Detail in
04-architecture.md and 07-tests.md.

## Salvageable assets

- `oklog/ulid/v2` choice (matches TS ULIDs) — keep, but generate
  UUIDv7 alongside for envelopes where the spec recommends time-
  ordered ids; `internal/ulid/` is a thin wrapper, retire it.
- `modernc.org/sqlite` (pure-Go, no CGo) for the event log — keep.
  Switch to TS's table shape: one row per envelope, `(session_id, id)`
  unique, secondary index on `(session_id, event_seq)`.
- `log/slog` — keep as the only logger. No `zap`/`zerolog` for a
  library.
- `transport.Transport` interface (`Send(ctx, env)` / `Recv(ctx)` /
  `Close()`) — keep; the envelope type changes underneath.
- The `examples/` directory pattern (one folder per scenario,
  `README.md` + Go source). Replace contents.
- The `auth.Verifier` interface — keep; rename to match v1.0
  `BearerVerifier` and drop the `MultiVerifier`/`AnonymousVerifier`
  extras (anonymous is not a v1.0 concern).

## Anti-salvage list (delete in the migration)

- `messages/control.go` (`ping`/`pong`/`ack`/`nack`/`cancel`/
  `interrupt`/`resume`/`backpressure`/`checkpoint.*` types) — none of
  these wire types exist in v1.0/v1.1. Pieces of the concepts are
  reborn as session messages (ping/pong, ack, cancel) or are not in
  scope (interrupt, backpressure as a wire type).
- `messages/streaming.go` (`stream.*`) — v1.0 has no streams. v1.1
  has `result_chunk` inside `job.event`.
- `messages/subscriptions.go` (generic `subscribe`/`subscribe.*`) —
  v1.1 has `job.subscribe`, scoped to one job, not a generic
  pub/sub.
- `messages/permissions.go` (`permission.*`, `lease.granted/extended/
  revoked/refresh`) — v1.0 leases are immutable; no revoke, no
  refresh.
- `messages/human.go` — out of scope per §1.2 (HITL is not ARCP).
- `messages/artifacts.go` envelope types (`artifact.put/fetch/release`)
  — v1.0 has only `artifact_ref` as a `job.event.kind`, not standalone
  envelopes.
- `messages/execution.go` (`tool.invoke`/`tool.result`/`tool.error`,
  `job.heartbeat`/`job.checkpoint`/`job.schedule`/`agent.handoff`,
  `workflow.*`) — none in v1.0/v1.1.
- `messages/telemetry.go` standalone (`log`, `metric`, `trace.span`,
  `event.emit`) envelope types — these become `job.event.kind` bodies.
- `internal/ulid/` — duplicates `oklog/ulid/v2`.

Leaving these in place during a partial migration creates a footgun
where two competing "lease" or "metric" surfaces co-exist. Cut once.

## Bottom line

Treat 02-current-audit.md as a closer. v1.1 planning files
03–09 should describe the destination, not a migration of this
codebase. The destination is a v1.0-clean module that additively
exposes v1.1; reaching it from here is a rewrite of envelope.go,
errors.go, the entire `messages/` package, `runtime/`, `client/`,
and `tests/`, with `transport/` and `store/` modified at the type
boundaries only.
