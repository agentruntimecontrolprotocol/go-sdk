# 03 — Libraries: dependency picks for the v1.1 Go SDK

One pick per concern. Every non-stdlib pick justifies itself against
a named alternative and cites a release within 18 months or stdlib-
grade status. The current `go.mod` at `../go.mod` declares
`go 1.25.0` and pulls only `oklog/ulid/v2` + `modernc.org/sqlite`
direct; this file revises that floor and adds three runtime concerns
not yet pinned (WebSocket, UUIDv7, OTel API).

## Minimum Go version

**Pick: `go 1.23.0`.** 1.23 ships `iter.Seq`/`iter.Seq2` and stable
range-over-func — the spec's §6.6 cursor pagination and §7.6
subscriber fan-out (see `02-current-audit.md` "high-risk items") both
read more cleanly as iterators than as channel-of-channel goroutine
trees. 1.22 would also work (matured `log/slog`, fixed loopvar) but
costs us the iterator surface. We do **not** pick 1.24/1.25 because
`go.mod`'s current `1.25.0` is aspirational — 1.25 is not the
oldest-supported Go for downstream consumers in May 2026 and pinning
it forces every importer to ship the bleeding-edge toolchain.

Citation: `../go.mod:3` (the value to overwrite); spec §6.6, §7.6.

## JSON

**Pick: stdlib `encoding/json`.** §8.4 `result_chunk` is the only hot
path; chunks are already capped at ~1 MB per §14 security guidance,
and a streaming `json.Encoder` writing one envelope per chunk is not
the bottleneck — WebSocket frame overhead and TLS dominate. Why over
`goccy/go-json`: extra dep, has had correctness regressions on
unknown-field passthrough, which §5.1 mandates (see `01-spec-delta.md`
on envelope forward-compat). Why not `encoding/json/v2`: it is
experimental in Go 1.23 and not GA — revisit when GA.

Citation: spec §5.1, §8.4, §14; current SDK `../envelope.go:88-108`
already routes through stdlib `json.RawMessage`.

## WebSocket

**Pick: `github.com/coder/websocket`** (formerly `nhooyr.io/websocket`).
Why over `gorilla/websocket`: context-cancellation propagates through
`Read`/`Write` natively, which is what §6.4 heartbeat watchdogs and
§7.6 subscriber fan-out need (`02-current-audit.md` flags goroutine
leaks if reads do not respect ctx). Gorilla requires `SetReadDeadline`
gymnastics and was in maintenance limbo 2022–2024. Why not
`fasthttp/websocket`: ties us to `fasthttp`'s non-`net/http` server
model; we want to attach to `http.Handler` for §4 `wss://` upgrade
inside any embedder. `coder/websocket` exposes `Accept(w, r)` for the
server-side upgrade and `Dial(ctx, url, opts)` client-side; close
semantics are `Close(code, reason)` returning a single status —
sufficient for the close codes referenced in spec §6.7 / §12.

Last release: `v1.8.13` (2025-04). Import:
`github.com/coder/websocket`.

## HTTP

**Pick: stdlib `net/http`.** Bearer auth (§6.1) is one header. Reject
`go-resty/resty`: a library does not get to install a global
`http.Client` or impose retry middleware on consumers — that is the
embedding application's call. Stdlib `http.Client` with a caller-
supplied `*http.Client` field on `client.Options` keeps testing
trivial (`httptest.Server`) and adds zero deps.

Citation: spec §6.1; current SDK has no HTTP client today
(`../client/` only wraps `transport.Transport`).

## Logging

**Pick: stdlib `log/slog`.** Library code emits via a `*slog.Logger`
field on `Client` / `Server`, default `slog.Default()`. Why over
`zap` / `zerolog`: a library imposing a logging framework on its
consumer is a packaging error — the embedder owns handler choice,
sampling, redaction, attr keys. `slog` gives us structured attrs
(`slog.String("session_id", id)`, `slog.Int64("event_seq", seq)`)
without binding to any backend. Zap and zerolog are application
loggers; both ship global state we cannot opt out of.

Citation: `02-current-audit.md` "Salvageable assets" already commits
to slog-only.

## IDs (ULID + UUIDv7)

Different id roles want different generators:

| Role          | Pick                      | Why                                                                |
| ------------- | ------------------------- | ------------------------------------------------------------------ |
| Envelope `id` | `github.com/google/uuid` `NewV7` | Spec §5.1 calls for time-ordered ids; UUIDv7 is RFC 9562 standard. |
| `session_id`  | `oklog/ulid/v2`           | Matches TS `packages/core` ULID emission; cross-SDK trace grepping. |
| `job_id`      | `oklog/ulid/v2`           | Same; appears in `job.subscribe` and audit logs (§7.6, §14).        |
| `result_id`   | `oklog/ulid/v2`           | Runtime-emitted (§8.4); ULID's lexicographic order helps log scans. |
| Ping `nonce`  | `oklog/ulid/v2`           | §6.4 `nonce` is matched on `pong`; ULID is short and time-ordered.  |

Why split: ULID matches the TS SDK's wire output (parity hint from
`02-current-audit.md`); UUIDv7 is the spec's preferred envelope id
type and `google/uuid` already sits in our indirect deps at v1.6.0
(`../go.mod:13`). Why not `gofrs/uuid`: redundant given `google/uuid`
1.6 ships `NewV7`; one fewer module.

Last releases: `oklog/ulid/v2 v2.1.1` (2025-02); `google/uuid v1.6.0`
(2024-01) — within 18 months for ULID, stdlib-grade for `google/uuid`
which is effectively frozen on the v7 surface.

Imports: `github.com/oklog/ulid/v2`, `github.com/google/uuid`.

## Tracing

**Pick: `go.opentelemetry.io/otel`** (API package only). v1.1 §11
adds the recommended attrs `arcp.lease.expires_at` and
`arcp.budget.remaining`; the library sets those on the active span if
one is present, otherwise no-ops. Why over bundling an SDK: the SDK
and exporters are the embedder's choice (OTLP, stdout, Jaeger). A
library that imports `go.opentelemetry.io/otel/sdk` forces consumers
into a specific resource/processor pipeline — see `01-spec-delta.md`
row 11: "OTel adapter when capabilities present" is a middleware
concern, not core.

Last release: `v1.37.0` (2025-08). Import:
`go.opentelemetry.io/otel` (and `go.opentelemetry.io/otel/trace`,
`go.opentelemetry.io/otel/attribute`).

## Errors

**Pick: stdlib `errors` + `fmt.Errorf("...: %w", err)`.** The 15
error codes in §12 become sentinel `var ErrXxx = errors.New(...)`
plus a `*ProtocolError` struct carrying `Code`, `Message`,
`Retryable`. `errors.Is`/`errors.As` covers everything
`01-spec-delta.md` row 12 needs (`arcp.IsRetryable`). Reject
`pkg/errors`: archived 2021, not stdlib-grade, and stdlib `%w`
chains replace its `Wrap`.

Citation: spec §12; `01-spec-delta.md` "New error codes" table.

## Testing

**Pick: stdlib `testing` + table-driven + `testify/require`** for
assertion ergonomics, plus `go.uber.org/goleak` per-package in
`TestMain`, plus `gotestsum` as CI reporter only (not a code
dep). Why `testify/require` over `matryer/is`: the high-risk areas
flagged in `02-current-audit.md` (subscribe fan-out, list_jobs
cursor) need deep struct asserts on event sequences; `require.Equal`
+ `go-cmp` diffs read better than `is.Equal`, and `require.NoError`
short-circuits before subsequent statements deref nil. `is` is fine
for one-liner units; we will have hundreds of envelope round-trip
asserts where short-circuit matters. `goleak.VerifyTestMain` is
non-negotiable given the goroutine-leak risk called out in
`02-current-audit.md` rows §6.4 and §7.6.

Last releases: `testify v1.10.0` (2024-11); `goleak v1.3.0`
(2023-10 — stdlib-grade test helper, no API churn expected);
`gotestsum v1.12.3` (2025-01, CI-only).

Imports: `github.com/stretchr/testify/require`,
`go.uber.org/goleak`.

## Fuzz

**Pick: stdlib `testing.F`.** Two targets:

1. Envelope parser — random JSON in, must either parse to a typed
   envelope or return a structured `INVALID_REQUEST`, never panic.
2. Agent-ref grammar (§7.5) — `name "@" version` per the BNF; the
   `ParseAgentRef` function must reject everything outside
   `[a-z0-9][a-z0-9._-]*` (name) / `[a-zA-Z0-9.+_-]+` (version)
   without panicking.

No deps. `testing.F` corpus persisted under `testdata/fuzz/`.

Citation: spec §7.5 grammar; `01-spec-delta.md` row §7.5.

## Diff

**Pick: `github.com/google/go-cmp/cmp`.** Used inside
`require.Equal`-comparator overrides and in custom assertions that
diff event streams. Reject `reflect.DeepEqual` for envelopes: it
treats `nil` slice vs empty slice as unequal and produces
no-context "false" failures — useless for diagnosing why two
event sequences diverged. `go-cmp` is stdlib-grade (Google-
maintained, semver-stable, dep-free).

Last release: `v0.7.0` (2025-01). Import:
`github.com/google/go-cmp/cmp`.

## Coverage

**Pick: `go test -cover -coverpkg=./...`.** Stdlib. CI gate at
**87%** floor (the bootstrap number called out in the task brief).
No HTML report tool in the library tree — `go tool cover -html` is
the developer-side reader. CI uploads `coverage.out` only.

## Lint

**Pick: `golangci-lint`** with this enabled set:

| Linter        | Why                                                                 |
| ------------- | ------------------------------------------------------------------- |
| `govet`       | Stdlib correctness; nilness, shift, copylocks.                      |
| `staticcheck` | SA series; replaces deprecated `megacheck`.                         |
| `errcheck`    | Catches dropped errors — critical for the `Send`/`Recv` loops.      |
| `revive`      | Drop-in for `golint`; configurable rule set, no maintenance gap.    |
| `gocritic`    | Style + perf checks (`rangeValCopy`, `appendAssign`).               |
| `gosec`       | Catches `crypto/rand` misuse, weak TLS configs (§4 `wss://`).       |
| `bodyclose`   | We hold `net/http` clients for bearer auth (§6.1) — must close.     |
| `unconvert`   | Removes no-op type conversions; tiny but cheap signal.              |
| `goimports`   | Import grouping + missing-import fix.                               |
| `gofumpt`     | Stricter `gofmt`; consistent formatting across contributors.        |

Excluded:

- `lll` — long-line linter; arbitrary. We respect ~100 cols by
  convention, not by tool. Mechanical wrapping of a JSON example
  string degrades readability.
- `varnamelen` — Go conventions prefer short receiver/loop names;
  this linter fights the language.
- `wsl`, `nlreturn` — whitespace-style noise.
- `exhaustive` — false positives on the 15 error-code switch in
  `errors.go` where a default arm is intentional.

`golangci-lint` is invoked via CI and a pre-commit hook; no
Go-module dependency.

## Format / build

- **`goimports`**: yes, replaces `gofmt` for the import block. Run
  on every save; enforced in CI by `golangci-lint`.
- **`gofumpt`**: yes — stricter `gofmt` superset; deterministic
  output so the lint check is not order-sensitive.
- **`go vet`**: yes, part of `golangci-lint` via `govet`.
- **`staticcheck`**: yes, via `golangci-lint` (do not double-pin).
- **`go build ./...`**: CI gate; the module must build with the
  declared `go 1.23.0` toolchain on linux/amd64, linux/arm64,
  darwin/arm64, windows/amd64.

---

## Hard rules (restated)

1. Minimum Go version is **1.23.0**, defended by `iter.Seq` for
   cursors (§6.6) and subscribers (§7.6). Not "latest Go".
2. Zero deps the stdlib covers cleanly: JSON, HTTP, logging,
   errors, testing, fuzz, coverage all stay stdlib.
3. No deps stale beyond 18 months unless stdlib-grade. All picks
   above shipped within 2024-01 → 2025-08 except `goleak` (stdlib-
   grade test helper, frozen API) and `google/uuid` (frozen v7
   surface).
4. Adapters (HTTP frameworks, OTel exporters, storage drivers
   beyond `modernc.org/sqlite`) are deferred to Phase 5. This file
   picks the WS library, not the host adapter library.

Final direct-dep list for `go.mod`:

```
github.com/coder/websocket          // §4 wss://
github.com/google/go-cmp            // tests
github.com/google/uuid              // envelope id (UUIDv7)
github.com/oklog/ulid/v2            // session/job/result/nonce ids
github.com/stretchr/testify         // tests
go.opentelemetry.io/otel            // §11 (api only)
go.opentelemetry.io/otel/trace      // §11
go.uber.org/goleak                  // tests
modernc.org/sqlite                  // event log (already in go.mod)
```

Nine direct deps, six of them test-only or single-call-site.
