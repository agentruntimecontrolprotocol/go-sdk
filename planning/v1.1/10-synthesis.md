# 10 — Synthesis

## Executive summary

The Go SDK does not implement ARCP v1.0. The in-tree `RFC-0001-v2.md`
describes a different protocol (different envelope, different
session/job verbs, different error taxonomy). Calling this a "v1.1
migration" is a category error: it is a **rewrite of the v1.0
surface** with v1.1 additions landed in the same release. The good
news is the rewrite is bounded and the destination is well-defined
by `../typescript-sdk/CONFORMANCE.md`.

Plan totals across phases 1–9:

| Phase | File                    | Lines | Headline                                                                                         |
| ----- | ----------------------- | ----- | ------------------------------------------------------------------------------------------------ |
| 1     | `01-spec-delta.md`      |    98 | 9 v1.1 features + 3 new error codes; all additive, all gated by `capabilities.features` (§6.2).  |
| 2     | `02-current-audit.md`   |   178 | 0/12 v1.0 normative requirements implemented; salvage transport interface, sqlite store, slog.   |
| 3     | `03-libraries.md`       |   264 | Go ≥ 1.23, `coder/websocket`, `google/uuid` + `oklog/ulid/v2`, stdlib JSON/HTTP/slog/errors.     |
| 4     | `04-architecture.md`    |   526 | Single module, root + 5 sub-packages, `<-chan Event` over `iter.Seq2`, dedicated writer goroutine. |
| 5     | `05-middleware.md`      |   317 | Three required adapters: `nethttp`, `chi`, `otel`; gin/echo/fiber deferred with stated unblocks. |
| 6     | `06-examples.md`        |   190 | 21 example dirs (drop `bun`/`express`/`fastify`; add `nethttp-routes`/`chi-routes`).             |
| 7     | `07-tests.md`           |   398 | 87% lines+stmts floor under `-race`, `goleak` per package, `testing.F` for envelope + agent ref. |
| 8     | `08-docs-readme.md`     |   368 | `docs/` tree with 6 top-level + 9 feature pages; `Example_xxx` for compile-checked snippets.     |
| 9     | `09-diagrams.md`        |   195 | 8 `.dot` diagrams covering package graph, session/job FSM, subscribe/heartbeat/result_chunk.     |

## Cross-phase contradictions (resolved here)

| Contradiction                                                                                                                  | Resolution                                                                                                                                                                       |
| ------------------------------------------------------------------------------------------------------------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Phase 4 ("§What this leaves to later phases") mentions `chi`, `gorilla/mux`, OTel as adapter set. Phase 5 picks `nethttp`, `chi`, `otel`; rejects `gorilla/mux`. | Phase 5 wins. `gorilla/mux` is in archive/community maintenance (Phase 5 cites this); not a Phase 11 adapter. Phase 4's list is updated by the synthesis here.                  |
| Phase 7 references `t.Context()` (Go 1.24+). Phase 3 sets floor at 1.23.                                                       | Phase 7 already documents the portable form: `ctx, cancel := context.WithCancel(context.Background()); t.Cleanup(cancel)`. Use this. Adopt `t.Context()` only when the floor moves to 1.24. |
| TS otel adapter puts `traceparent` in `extensions["x-vendor.opentelemetry.tracecontext"]`; my Phase 5 brief said "`trace_id` field". | Phase 5 resolved correctly: `trace_id` per §11 is the bare 32-hex trace id alone; the full W3C `traceparent` carrier rides in the extensions key. Both peers populate both fields. |
| Phase 7 mentions injected `clock.Mock` for time control. Phase 3 did not pick a clock lib.                                     | Define a small unexported `Clock` interface where consumed (`internal/lease`, `server.heartbeat`), with `realClock` and a test-only `mockClock`. No third-party clock dep. Listed under open questions if a community lib is preferred later. |
| Phase 4 says heartbeat ticker is "per session per direction." Phase 5/7 imply a single ticker that watches inbound idle on a shared timestamp. | Two timers per session: outbound idle ticker (emits `session.ping`) and inbound watchdog (`time.AfterFunc(2 × interval, surface HEARTBEAT_LOST)` reset on every inbound envelope). One direction-per-timer; no shared timestamp. Phase 4 wording stands. |

No other cross-phase contradictions found.

## Ordered, PR-sized milestones

Each milestone is one reviewable PR unless marked "split". "Spec"
column is the section(s) of `../spec/docs/draft-arcp-02.1.md` the PR
exercises end-to-end. "Files" lists primary targets (not exhaustive).

| #   | Milestone                                | Spec               | Files                                                                                              | Depends on |
| --- | ---------------------------------------- | ------------------ | -------------------------------------------------------------------------------------------------- | ---------- |
| M0  | Excise `RFC-0001-v2` surface             | —                  | delete `messages/{control,streaming,subscriptions,permissions,human,artifacts,telemetry,execution,session}.go`, `runtime/runtime.go` body, `client/client.go` body, all 14 `examples/` dirs, `cmd/arcp/` body, `internal/ulid/`. Update `go.mod` to `go 1.23.0`, drop `modernc.org/sqlite` if Phase 11 keeps it out of root deps. Replace `README.md` and `CONFORMANCE.md` with v1.1 placeholders. | —          |
| M1  | Envelope + errors + ids + features      | §5, §12            | new `envelope.go`, `errors.go` (15 codes), `version.go` (`ProtocolVersion = "1"`), `internal/version/features.go`, `internal/idstore` helpers around `uuid.NewV7` and `ulid.MustNew`. Round-trip tests + `testing.F` seed corpus from spec §13. | M0         |
| M2  | Typed payloads — v1.0 message set       | §5–§10             | `messages/{session,job,event}.go` for v1.0 (`session.hello`, `session.welcome`, `session.bye`, `session.error`, `job.submit`, `job.accepted`, `job.event`, `job.result`, `job.error`, `job.cancel`). Registry via `init()`. | M1         |
| M3  | Transport layer                          | §4                 | `transport/transport.go` (interface kept), `transport/memory.go` (port), `transport/websocket.go` (`coder/websocket`), `transport/stdio.go` (NDJSON). Per-transport unit + goleak tests. | M1         |
| M4  | Server v1.0 happy path                   | §6, §7.1–§7.4, §8  | `server/server.go`, `server/session.go`, `server/job.go`, `server/jobctx.go`. Auth via `auth/bearer.go`. Event log under `internal/eventlog/` (keep `modernc.org/sqlite`).                       | M2, M3     |
| M5  | Client v1.0 happy path                   | §6, §7, §8         | `client/client.go`, `client/handle.go`. `Connect`, `Submit`, `Wait`, `Events()`, `Cancel`, `Close`. Resume via `client.Resume(...)` honouring §6.3.                                              | M4         |
| M6  | Lease + delegation                       | §9, §10            | `internal/lease/{lease,glob,canonical,validate}.go`. Delegation interceptor in `server/`. `LEASE_SUBSET_VIOLATION` surfaced as `tool_result` on parent per §10.2.                                | M4, M5     |
| M7  | v1.0 conformance harness                 | §4–§15             | `tests/conformance/*_test.go` keyed to `CONFORMANCE.md`; JSON summary diffed against `../typescript-sdk/`'s output. Coverage gate raised to 87% at the end of this PR.                          | M5, M6     |
| M8  | v1.1 §6.2 capability negotiation         | §6.2               | `messages/session.go` adds `Features []string`, rich `agents` shape; `internal/features.Intersect`; `Client.HasFeature` / `Session.HasFeature`. No new wire types — wires the gate for M9–M14. | M7         |
| M9  | v1.1 §6.4 heartbeat + §6.5 ack          | §6.4, §6.5         | `messages/session.go` `Ping`/`Pong`/`Ack` payloads; `server/heartbeat.go`; `client/ack.go` with coalescer; back-pressure threshold in `server/`. Integration test asserts `HEARTBEAT_LOST` with injected `Clock`. | M8         |
| M10 | v1.1 §7.5 agent versioning              | §7.5, §12          | `messages/agentref.go` with `Parse`/`Format`; `server.RegisterAgentVersion` + `SetDefaultAgentVersion`; `AGENT_VERSION_NOT_AVAILABLE` raise site. Fuzz on `Parse`.                              | M8         |
| M11 | v1.1 §6.6 list_jobs + §7.6 subscribe    | §6.6, §7.6, §12    | **Split into two PRs.** (a) `list_jobs` with opaque cursor `base64(JSON{after_id, after_created_at})`; same-principal auth. (b) `subscribe` with per-subscriber `event_seq` allocation; bounded fan-out channel; cancel-denied path. | M8         |
| M12 | v1.1 §9.5 expires_at + §9.6 cost.budget | §9.5, §9.6, §12    | `internal/lease/validate.go` takes `now Clock` and `budget map[Currency]float64`; watchdog via `time.AfterFunc`; `Job.applyCostMetric` debits per `metric` event. Negative values rejected.    | M6, M8     |
| M13 | v1.1 §8.2.1 progress + §8.4 result_chunk | §8.2.1, §8.4      | `messages/event_progress.go`, `messages/event_result_chunk.go`; `JobContext.Progress`, `JobContext.StreamResult` returning an `io.WriteCloser`-style writer; client `JobHandle.Chunks() <-chan ResultChunk` + `CollectChunks` helper. MUST-NOT-mix enforcement. | M5         |
| M14 | Middleware: `nethttp` + `chi`           | §4, §14            | `middleware/nethttp/` + `middleware/chi/`. `allowedHosts` default `["localhost", "127.0.0.1", "[::1]"]`; `421 Misdirected Request` on rebind; conn-tracking for graceful `Shutdown(ctx)` (named deadlock per Phase 5). | M3         |
| M15 | Middleware: `otel`                      | §11                | `middleware/otel/`. `arcp.lease.expires_at`, `arcp.budget.remaining` attrs; `traceparent` carrier in `extensions["x-vendor.opentelemetry.tracecontext"]`. Wraps both `Client` and `Server`.    | M9–M13     |
| M16 | Examples (21 dirs)                      | §13 + host         | `examples/{submit-and-stream,delegate,resume,idempotent-retry,lease-violation,cancel,stdio,vendor-extensions,custom-auth,heartbeat,ack-backpressure,list-jobs,subscribe,agent-versions,lease-expires-at,cost-budget,progress,result-chunk,tracing,nethttp-routes,chi-routes}`. Ports 7811–7831. `Makefile :: examples-smoke`. | M5–M15     |
| M17 | Docs + diagrams                         | —                  | `docs/` tree per Phase 8 (`00-overview`, `01-quickstart`, `02-concepts`, `03-features/{9}`, `04-examples/{21}`, `05-reference/{6}`, `06-conformance`). `docs/diagrams/*.dot` per Phase 9 with `render.sh`, `Makefile :: diagrams`, `diagrams.lock` content hash. | M16        |
| M18 | Tag v1.1.0                              | —                  | `CHANGELOG.md`, `go.mod` minor, tag, release notes. Conformance harness output published as the release artifact.                                                                              | M17        |

Three milestones are explicitly split for review hygiene:
- **M2** can split per message group if the diff exceeds ~1500 LoC.
- **M11** splits `list_jobs` and `subscribe` — both are high-risk Go problems (cursor re-entrancy, subscriber fan-out goroutine leak).
- **M16** can land example-per-PR alongside the feature it exercises; the column shows "M5–M15" because the example for each feature should land in the same PR as the feature.

Total: ~22 PRs (counting splits) over an estimated 6–10 weeks of
focused work for one engineer, less with two.

## Risks

| Risk                                                                                                                                                                                                                                | Mitigation                                                                                                                                              |
| ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Subscriber fan-out goroutine leak (§7.6).** Per-subscriber `<-chan Event` allocated on a goroutine; if the subscriber drops without `Close()`, the goroutine blocks on `chan<- Event` send forever and pins the owning session's emitter. | `goleak.VerifyTestMain` per package (Phase 7); bounded channel + non-blocking select with timeout drop; subscriber unsubscribe is `context.AfterFunc`. |
| **List-jobs cursor non-reentrancy under `ctx.Cancellation`.** Cursor decode + DB scan must check `ctx.Err()` between pages or a cancelled request leaks a transaction.                                                              | Opaque cursor is content-addressable (`base64(JSON{after_id, after_created_at})`); pagination is stateless on the server; each page call is its own short transaction with `ctx`-bound deadline. |
| **WebSocket read loop returning before send channel drained** (the classic `coder/websocket` footgun).                                                                                                                              | Shutdown order codified in Phase 4: cancel `session.ctx` → wait ticker → drain writer outbox → close transport. Test under `-race` with goleak.        |
| **Budget counter race (§9.6).** Two concurrent `validateLeaseOp` calls each see counter > 0, both authorize, both debit; counter goes negative without the second call being rejected.                                              | Per-currency `sync.Mutex` (not `atomic.Int64` — debit-and-check must be one critical section). 64-goroutine race test in Phase 7.                       |
| **Heartbeat ticker / `Close` deadlock.** Ticker goroutine writes to outbox; `Close` waits for ticker; both block.                                                                                                                   | Shutdown sequence is: cancel `session.ctx` first (ticker exits on ctx.Done); only then close outbox; close transport last.                              |
| **Lease-expiry clock skew.** `time.Now()` is wall clock; NTP adjustments and tests with injected clocks must agree on a single source.                                                                                              | Inject `Clock` interface (synthesis decision above). All `expires_at` arithmetic flows through it. Production wires `time.Now`; tests wire a mock.       |
| **`result_chunk` memory exhaustion (§14).** Unbounded chunk sizes blow the heap on assembly.                                                                                                                                       | Phase 5 sets `ReadLimit = 1 << 20`; runtime caps per-result total size; `INTERNAL_ERROR` on exceed (§14). Test with a deliberately oversized chunk.   |
| **Existing `examples/` directories are confusing during M0–M5.** Reviewers may assume the deleted scenarios are intentional regressions.                                                                                            | M0 commit message lists every deleted directory and says "deleted because v0.1 implemented `RFC-0001-v2`, not v1.0. The v1.1 examples land in M16."     |
| **`go.mod` still says `go 1.25.0`.** That toolchain doesn't exist as of the spec date (May 2026). Anything built against it fails to compile on community Go.                                                                       | M0 fixes this. The floor is 1.23 (Phase 3).                                                                                                              |

## Non-goals (state and stop)

- **No backport to v1.0 SDK.** No v1.0 Go SDK was ever published
  against this codebase's `RFC-0001-v2` design. There is no v1.0
  release line to maintain.
- **No pause/unpause, priority/scheduling hints, federation,
  streaming-token surface.** Spec §"Not in v1.1".
- **No HITL (human-in-the-loop) surface.** §1.2 non-goal of the
  protocol. `messages/human.go` is deleted in M0 and not replaced.
- **No `agent.handoff` or `workflow.*`.** Not in v1.0/v1.1.
- **No `gin`/`echo`/`fiber` adapter for v1.1.** Phase 5 documents the
  unblock criteria; revisit in v1.2.
- **No `hono`/`bun` adapter.** No Go analogue (Phase 5).
- **No persistent idempotency store.** TS ships an in-memory store
  with a 24h TTL sweep; Go matches (`internal/idstore`). Production
  deployments override.
- **No mTLS or OAuth2 auth.** v1.0/v1.1 specifies bearer; everything
  else is deployment policy on top of bearer (Phase 4 §Auth).

## Open questions (track in issues, not in code)

1. **Clock interface location.** Synthesis says `internal/lease` and
   `server/heartbeat` define a local `Clock` interface. Should this
   be promoted to a single `internal/clock` package shared across
   the two consumers? Decision deferred until the second consumer
   needs it; one place is fine for now (interfaces defined where
   consumed).
2. **`encoding/json/v2` adoption.** Phase 3 rejects it on
   experimental status. When does it go GA? Track Go release notes;
   revisit in v1.2.
3. **`iter.Seq2[Event, error]` versus `<-chan Event` + `Subscription.Err()`.**
   Phase 4 chose the channel form to keep the floor at 1.23.
   Reconsider when the floor moves to 1.24 — the iterator surface
   is more ergonomic for `for ev := range sub.Events()` callers but
   forces a `func() (T, bool)` adapter shape that is awkward for
   error propagation.
3. **Conformance harness format alignment with TS.** Both SDKs emit
   JSON, but the TS shape isn't formalized. Open question: agree a
   shared schema in `../spec/conformance.schema.json` so cross-SDK
   diffs work. Pre-requisite for the M18 release artifact claim.
4. **Diagram dark-mode rendering.** Phase 9 chose to render dark
   from the same `.dot` via overrides; TS keeps a second `.dot`. If
   review feedback prefers the TS approach, swap during M17 —
   contained change.
5. **CLI surface.** `cmd/arcp/` is empty in M0. The TS `@arcp/sdk`
   ships `serve`, `submit`, `replay`. Decide before M5 whether the
   Go CLI ships in v1.1.0 or in v1.1.1. Recommend v1.1.1: the
   library surface is the v1.1 contract; the CLI is convenience.

## What good looks like at v1.1.0

- `go get github.com/agentruntimecontrolprotocol/go-sdk@v1.1.0`
  builds clean against Go 1.23 with zero indirect dep churn from the
  current state.
- `go test -race -cover -coverpkg=./... -covermode=atomic ./...`
  passes with ≥87% coverage on every package; `goleak` clean per
  package.
- `tests/conformance/` emits a JSON summary with zero "missing"
  rows for §4–§15 and for the v1.1 additions §6.2/§6.4/§6.5/§6.6/
  §7.5/§7.6/§8.2.1/§8.4/§9.5/§9.6/§11/§12.
- `examples/` smoke runs all 21 directories in CI and asserts exit
  code 0 per example.
- `pkg.go.dev/github.com/agentruntimecontrolprotocol/go-sdk` renders
  with `Example_xxx` runnable on every package.
- `CONFORMANCE.md` matches the live harness output; CI fails the PR
  if they drift.

## What to do next

Open M0 as the first PR. Title: "delete RFC-0001-v2 surface;
reset go.mod floor to 1.23." Body: link to `02-current-audit.md`
and quote the section listing wire-level divergences. Reviewers
will need that context to approve a deletion this large.
