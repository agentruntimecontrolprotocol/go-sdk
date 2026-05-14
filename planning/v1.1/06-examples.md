# 06 — Examples Mapping (Go)

One Go example directory per TypeScript example, minus the
JS-only host integrations (`express/`, `fastify/`, `bun/`), plus
two Go-side host integrations (`chi-routes/`, `nethttp-routes/`).
Final count: **20 example directories** (9 v1.0 core + 9 v1.1
features + 2 Go host integrations) plus `tracing/`, total **21**.

The current `examples/` directory holds 14 dirs targeting the wrong
in-repo `RFC-0001-v2` protocol (see `02-current-audit.md`). All
14 are deleted in this phase; nothing in them survives the wire
rewrite.

## Mapping table

| TS dir              | Go dir              | files                                                  | spec §             | Go idiom shown off                                                                                                                                                                                  | run command                                                                                       |
| ------------------- | ------------------- | ------------------------------------------------------ | ------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------- |
| `submit-and-stream` | `submit-and-stream` | `server/main.go`, `client/main.go`, `README.md`        | §13.1, §7.1, §8.2  | client ranges `for ev := range handle.Events()` (a `<-chan arcp.Event` closed at terminal); switches on `ev.Kind`; the loop exits naturally when the channel closes after `job.result`              | `go run ./examples/submit-and-stream/server` / `go run ./examples/submit-and-stream/client`       |
| `delegate`          | `delegate`          | same                                                   | §13.2, §10         | parent agent calls `jc.Delegate(ctx, arcp.DelegateRequest{...})`; child `JobContext.TraceID()` equals parent's; child lease `cost.budget` asserted via `arcp.LeaseIsSubset(child, parent.Remaining())` | `go run ./examples/delegate/server` / `go run ./examples/delegate/client`                         |
| `resume`            | `resume`            | same                                                   | §13.3, §6.3        | client kills its WS conn mid-stream via `conn.Close()`; reconnects with `arcp.DialOptions{Resume: &arcp.ResumeRequest{SessionID: sid, ResumeToken: tok, LastEventSeq: n}}`; asserts only events with `seq > n` arrive on the new channel; new `resume_token` recorded | `go run ./examples/resume/server` / `go run ./examples/resume/client`                             |
| `idempotent-retry`  | `idempotent-retry`  | same                                                   | §13.5, §7.2        | two `Submit` calls with the same `arcp.SubmitOptions{IdempotencyKey: "k"}` return equal `handle.JobID()`; third call with same key + different agent returns `err` where `arcp.Code(err) == arcp.CodeDuplicateKey` | `go run ./examples/idempotent-retry/server` / `go run ./examples/idempotent-retry/client`         |
| `lease-violation`   | `lease-violation`   | same                                                   | §13.4, §9.3        | agent calls `jc.ToolCall(ctx, "fs.write", args)` outside lease; client receives the `job.event` with `payload.kind == "tool_result"`, body `error.code == "PERMISSION_DENIED"`; loop continues until `job.result` | `go run ./examples/lease-violation/server` / `go run ./examples/lease-violation/client`           |
| `cancel`            | `cancel`            | same                                                   | §7.4               | agent watches `select { case <-ctx.Done(): return ctx.Err() }`; no sentinel error sniffing — `context.Canceled` propagates; runtime emits `job.error{final_status:"cancelled"}` and the client's event channel closes | `go run ./examples/cancel/server` / `go run ./examples/cancel/client`                             |
| `stdio`             | `stdio`             | `agent/main.go`, `client/main.go`, `README.md` (no `server/` — runtime is in-process within the agent child) | §4.2, §22 | client uses `exec.CommandContext(ctx, "go", "run", "./agent")` then wraps the resulting `*exec.Cmd` stdin/stdout pair in `arcp.NewStdioTransport(stdout, stdin)`; framing is `bufio.Scanner` over NDJSON | `go run ./examples/stdio/client`                                                                  |
| `vendor-extensions` | `vendor-extensions` | `server/main.go`, `client-naive/main.go`, `client-aware/main.go`, `README.md` | §8.2, §9.2, §15 | server emits `payload.kind == "x-vendor.acme.progress"`; naive client falls through default branch of the `switch ev.Kind`; vendor-aware client uses `arcp.DecodePayload[acme.Progress](ev)` to render | `go run ./examples/vendor-extensions/server` / `go run ./examples/vendor-extensions/client-aware` |
| `custom-auth`       | `custom-auth`       | `server/main.go`, `client/main.go`, `README.md`        | §6.1               | server-side `auth.BearerVerifier` interface implemented by a `hmacVerifier` struct (stateless HMAC over `principal|exp`); rejected handshake surfaces `arcp.Code(err) == arcp.CodeUnauthenticated` | `go run ./examples/custom-auth/server` / `go run ./examples/custom-auth/client`                   |
| `heartbeat`         | `heartbeat`         | `server/main.go`, `client/main.go`, `README.md`        | §6.4               | server constructs `server.New(server.Options{HeartbeatInterval: 5*time.Second})`; client prints `welcome.HeartbeatIntervalSec` on connect; both peers fire `session.ping` from a `time.Ticker`; an idle-11s stretch assertion is commented (TODO: wire `arcp.CodeHeartbeatLost`) | `go run ./examples/heartbeat/server` / `go run ./examples/heartbeat/client`                       |
| `ack-backpressure`  | `ack-backpressure`  | `server/main.go`, `client/main.go`, `README.md`        | §6.5, §8.2         | client deliberately sleeps `time.Sleep(50*time.Millisecond)` inside the event loop; server's `AckLagThreshold` trips and emits a `payload.kind == "status"` with `body.phase == "back_pressure"`; client asserts that exact status before the result | `go run ./examples/ack-backpressure/server` / `go run ./examples/ack-backpressure/client`         |
| `list-jobs`         | `list-jobs`         | `server/main.go`, `client/main.go`, `README.md`        | §6.6               | client calls `sess.ListJobs(ctx, arcp.ListJobsRequest{Status: []string{"running"}, Agent: "echo", Limit: 10})`; loops `for cursor := ""; ; cursor = resp.NextCursor` until `cursor == ""` after the request returned `""`; both filters honoured | `go run ./examples/list-jobs/server` / `go run ./examples/list-jobs/client`                       |
| `subscribe`         | `subscribe`         | `server/main.go`, `client-submitter/main.go`, `client-observer/main.go`, `README.md` | §7.6, §6.6 | observer process (same principal token, different `session_id`) calls `client.Subscribe(ctx, jobID, arcp.SubscribeOptions{History: true, FromEventSeq: 0})`; cross-session cancel path asserts `arcp.Code(err) == arcp.CodePermissionDenied` | `go run ./examples/subscribe/server` / two clients in two terminals                               |
| `agent-versions`    | `agent-versions`    | `server/main.go`, `client/main.go`, `README.md`        | §7.5, §12          | `server.RegisterAgentVersion("code-refactor", "1.0.0", v1Fn)` and `("code-refactor", "2.0.0", v2Fn)`; `server.SetDefaultAgentVersion("code-refactor", "2.0.0")`; client submits `"code-refactor"`, `"code-refactor@1.0.0"`, `"code-refactor@3.0.0"`; third surfaces `arcp.Code(err) == arcp.CodeAgentVersionNotAvailable` | `go run ./examples/agent-versions/server` / `go run ./examples/agent-versions/client`             |
| `lease-expires-at`  | `lease-expires-at`  | `server/main.go`, `client/main.go`, `README.md`        | §9.5, §12          | client submits with `LeaseConstraints{ExpiresAt: time.Now().Add(5*time.Second).UTC()}`; agent sleeps past it; both the agent-side `jc.ValidateLeaseOp` and the runtime watchdog raise; client asserts `arcp.Code(err) == arcp.CodeLeaseExpired` | `go run ./examples/lease-expires-at/server` / `go run ./examples/lease-expires-at/client`         |
| `cost-budget`       | `cost-budget`       | `server/main.go`, `client/main.go`, `README.md`        | §9.6, §12          | agent calls `jc.Metric(ctx, "cost.search", 0.42, "USD", nil)` in a loop; client observes `cost.budget.remaining` metric decrementing; final loop iteration returns a `tool_result` whose `body.error.code == "BUDGET_EXHAUSTED"` | `go run ./examples/cost-budget/server` / `go run ./examples/cost-budget/client`                   |
| `progress`          | `progress`          | `server/main.go`, `client/main.go`, `README.md`        | §8.2.1             | agent calls `jc.Progress(ctx, current, arcp.ProgressOpts{Total: 100, Units: "files"})`; client uses `fmt.Fprintf(os.Stdout, "\r[%-40s] %d/%d", bar, p.Current, p.Total)` with carriage return for in-place updates | `go run ./examples/progress/server` / `go run ./examples/progress/client`                         |
| `result-chunk`      | `result-chunk`      | `server/main.go`, `client/main.go`, `README.md`        | §8.4               | agent does `w := jc.StreamResult(ctx, arcp.StreamResultOptions{Encoding: "utf8"})`; each `w.Write(p)` emits a `result_chunk`; `w.Close()` produces the terminating `job.result` with `result_id` + `result_size`; client calls `bytes, err := handle.CollectChunks(ctx)` which assembles by `result_id` | `go run ./examples/result-chunk/server` / `go run ./examples/result-chunk/client`                 |
| `tracing`           | `tracing`           | `server/main.go`, `client/main.go`, `README.md`        | §11                | both sides wrap their `arcp.Client` / `server.Server` with `middleware/otel.New(tp)`; trace context rides in `extensions["x-vendor.opentelemetry.tracecontext"]`; spans printed via OTel `stdouttrace.New(stdouttrace.WithPrettyPrint())` | `go run ./examples/tracing/server` / `go run ./examples/tracing/client`                           |
| `express` (skip)    | `nethttp-routes`    | `server/main.go`, `client/main.go`, `README.md`        | §4.1               | runtime mounted at `/arcp` on a `*http.ServeMux` via `middleware/nethttp.Mount(mux, "/arcp", srv)`; `/healthz` and `/api/echo` are plain JSON handlers on the same mux; `srv.AllowedHosts = []string{"127.0.0.1", "localhost"}` for DNS-rebind protection | `go run ./examples/nethttp-routes/server` / `go run ./examples/nethttp-routes/client`             |
| `fastify` (skip)    | `chi-routes`        | `server/main.go`, `client/main.go`, `README.md`        | §4.1               | `r := chi.NewRouter(); r.Use(middleware.RequestID); middleware/chi.Mount(r, "/arcp", srv)`; chi's `middleware.RequestID` propagates into the ARCP request log; the example also mounts `r.Get("/jobs", ...)` to show coexistence | `go run ./examples/chi-routes/server` / `go run ./examples/chi-routes/client`                     |
| `bun` (skip)        | — (dropped)         | —                                                      | —                  | Bun is a JS runtime; no Go analogue. Coverage of "non-default HTTP server" is already provided by `nethttp-routes` and `chi-routes`.                                                                | —                                                                                                 |

22 TS examples → 20 Go examples + 1 dropped (`bun/`) with 2 added
host integrations (`chi-routes/`, `nethttp-routes/`). The TS README's
header count of 23 includes `tracing/` separately, which the table
above does too.

## Runner

`Makefile` target, not a shell script. Defence: a `Makefile` already
exists in the module root for `make test` / `make lint` /
`make conformance` per `03-libraries.md`; adding `examples-smoke`
keeps the developer-facing surface uniform, gives free `--keep-going`
behaviour with `-k`, and avoids `examples/run.sh` accruing per-OS
shellisms. The script form is harder to wire into CI in parallel.

```make
examples-smoke:
        @go run ./internal/cmd/examplesmoke
```

`internal/cmd/examplesmoke` is a small Go program that:

1. Lists every directory under `examples/` with a `server/main.go`.
2. For each, allocates the example's pinned port (table below),
   `exec.Command`s `go run ./examples/<name>/server` with that
   `ARCP_DEMO_PORT`, then `go run ./examples/<name>/client`.
3. Asserts client exit code is 0 within a 30s deadline; SIGINTs the
   server; collects stderr on failure.
4. `stdio/` is special-cased: only the client is run, since it spawns
   the agent itself.
5. Returns non-zero if any example fails. Exit summary prints a
   table of pass/fail per example.

Smoke run takes ~90s on a laptop because most examples sleep ≤5s.

## Common harness

```
examples/<name>/
  server/main.go      30–60 lines: build a Server, register one agent, listen on ws://127.0.0.1:$PORT/arcp
  client/main.go      30–60 lines: dial, submit, exercise the feature, assert with log.Fatal on mismatch, exit 0
  README.md            5–15 lines: what it shows, the spec § it cites, run command, expected stdout fragment
```

Everything is under one Go module (`github.com/anthropics/arcp-go`)
rooted at the SDK root — no per-example `go.mod`. Each example
imports `arcp`, `arcp/client`, `arcp/server`, and at most one
middleware package.

Server entry point uses `server.Listen(ctx, server.ListenOptions{Addr: ":"+port})`
which returns when `ctx` is cancelled; on SIGINT the example
exits 0. Client entry point uses `log.Fatal(err)` for any assertion
failure — there is no testing framework hookup, the smoke runner
reads exit codes.

### Port allocation

Each example listens on a unique loopback port so the smoke runner
can run them in parallel without collisions and a developer can
leave several `server` processes up simultaneously.

| Example             | Port | Env var          |
| ------------------- | ---- | ---------------- |
| `submit-and-stream` | 7811 | `ARCP_DEMO_PORT` |
| `delegate`          | 7812 | `ARCP_DEMO_PORT` |
| `resume`            | 7813 | `ARCP_DEMO_PORT` |
| `idempotent-retry`  | 7814 | `ARCP_DEMO_PORT` |
| `lease-violation`   | 7815 | `ARCP_DEMO_PORT` |
| `cancel`            | 7816 | `ARCP_DEMO_PORT` |
| `stdio`             | —    | n/a (pipe)       |
| `vendor-extensions` | 7818 | `ARCP_DEMO_PORT` |
| `custom-auth`       | 7819 | `ARCP_DEMO_PORT` |
| `heartbeat`         | 7820 | `ARCP_DEMO_PORT` |
| `ack-backpressure`  | 7821 | `ARCP_DEMO_PORT` |
| `list-jobs`         | 7822 | `ARCP_DEMO_PORT` |
| `subscribe`         | 7823 | `ARCP_DEMO_PORT` |
| `agent-versions`    | 7824 | `ARCP_DEMO_PORT` |
| `lease-expires-at`  | 7825 | `ARCP_DEMO_PORT` |
| `cost-budget`       | 7826 | `ARCP_DEMO_PORT` |
| `progress`          | 7827 | `ARCP_DEMO_PORT` |
| `result-chunk`      | 7828 | `ARCP_DEMO_PORT` |
| `tracing`           | 7829 | `ARCP_DEMO_PORT` |
| `nethttp-routes`    | 7830 | `ARCP_DEMO_PORT` |
| `chi-routes`        | 7831 | `ARCP_DEMO_PORT` |

Block `7811-7831` is reserved. The TS examples use `7700-7720`;
keeping the Go block disjoint lets the same machine host both SDKs'
smoke runs at once during cross-SDK conformance.

## What is NOT covered

These directories exist in the current Go `examples/` tree (the
RFC-0001-v2 surface) and have **no** counterpart in the v1.1 example
set:

- **No `human/` example.** `messages/human.go` defines a
  `human.input.request` / `human.input.response` round-trip, but
  ARCP §1.2 lists "human-in-the-loop UI flows" as a non-goal. HITL
  is the host application's job; ARCP carries the agent↔runtime
  channel only.
- **No `handoff/` example.** Agent-to-agent handoff with state
  transfer is not in §10 (delegation, which is a parent-spawning-child
  pattern). The `messages/execution.go:agent.handoff` envelope has
  no spec mapping.
- **No `permission_challenge/` example.** §6.1 specifies bearer
  auth at handshake; mid-session permission challenges
  (`permission.challenge` / `permission.response` in
  `messages/permissions.go`) are not in the spec. Capability changes
  are immutable per job (§9.1).
- **No `lease_revocation/` example.** §9.1 makes leases immutable
  for the job's lifetime; there is no revoke verb. The closest
  v1.1 mechanism is `expires_at` (covered by `lease-expires-at/`).
- **No `capability_negotiation/` example.** Feature negotiation
  is exercised implicitly by every v1.1 example that uses a
  capability-gated feature (`heartbeat/`, `ack-backpressure/`,
  `list-jobs/`, `subscribe/`, `agent-versions/`, `lease-expires-at/`,
  `cost-budget/`, `progress/`, `result-chunk/`). A standalone
  "negotiation" example would only print the `welcome.capabilities.features`
  array; the existing examples each assert on a concrete consequence
  of negotiation, which is more useful.
- **No `checkpoint/` example.** Current SDK has
  `messages/execution.go:job.checkpoint`; neither v1.0 nor v1.1
  defines a checkpoint verb. The corresponding behaviour for
  long jobs is `resume/` (replay from `event_seq`) plus the
  agent's own state persistence — both out of band.
- **No `mcp/` example.** Model Context Protocol bridging is a
  separate effort (see `arpc/spec` issue tracker). Not in v1.1.
- **No `extensions/` example as a freestanding directory.**
  Vendor extensions are exercised by `vendor-extensions/`; namespaced
  envelope passthrough is exercised by `tracing/` (which rides
  `extensions["x-vendor.opentelemetry.tracecontext"]`).
- **No `reasoning_streams/` example.** The §8.2 `thought` event
  kind is part of `submit-and-stream/`; a dedicated example would
  duplicate that coverage.
- **No JS host integrations.** `express/`, `fastify/`, `bun/`
  are skipped — the Go-side analogues are `nethttp-routes/`
  and `chi-routes/`. Echo and Gin are intentionally omitted to
  keep the dependency surface small; the chi example demonstrates
  the integration seam any third-party router would use
  (`http.Handler` mount).

## Phase outputs consumed by later phases

- Phase 7 (test plan) will use this table to scaffold per-example
  smoke tests under `internal/cmd/examplesmoke`.
- Phase 8 (delivery checklist) will treat each row as one PR-sized
  unit of work; the 20 example directories are independent commits.
- The cross-SDK conformance harness (TS ↔ Go interop) will pick
  three rows — `submit-and-stream`, `resume`, `subscribe` — as its
  smoke set, run with the TS server against the Go client and vice
  versa.
