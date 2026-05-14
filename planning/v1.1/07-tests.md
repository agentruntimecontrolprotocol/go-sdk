# 07 — Test plan: ARCP v1.1 Go SDK

Source spec: `../spec/docs/draft-arcp-02.1.md`. Mirror: the TS
suite at `../typescript-sdk/packages/*/test/` (vitest `describe/it`
style — Go translation is `t.Run` sub-tests). Package layout per
`04-architecture.md`: root `arcp`, plus `client/`, `server/`,
`transport/`, `messages/`, `auth/`, `middleware/{nethttp,chi,otel}`.

## Test stack

Driver is stdlib `testing`. No JUnit, no Ginkgo, no `testing/quick`.

Table-driven tests with sub-tests are the default for units. The
canonical shape is six lines:

```go
for _, tc := range cases {
    t.Run(tc.name, func(t *testing.T) {
        got, err := messages.ParseEnvelope(tc.in)
        require.NoError(t, err)
        require.Empty(t, cmp.Diff(tc.want, got))
    })
}
```

Assertion library: `github.com/stretchr/testify/require` (per Phase
3 pick in `03-libraries.md`; Phase 3 may swap to `matryer/is`, this
file keeps `require` as the default). `require` over `assert`: the
failure mode of `assert.Equal(t, want, got); doSomething(got)` is
that a falsy first assertion does not stop the test, so a later
nil-deref panic in `doSomething` masks the real diff. `require.Equal`
calls `t.FailNow`; the diff is the last line printed.

Struct equality: `github.com/google/go-cmp/cmp` plus
`google/go-cmp/cmp/cmpopts`. Reject `reflect.DeepEqual` in tests —
it lies on `time.Time` values whose monotonic clock components
differ (two times equal by wall clock compare unequal under DeepEqual
when one came back through JSON), on `math.NaN()` (every NaN is
DeepEqual-unequal to every NaN, including itself), and on
`sync.Mutex` zero value (a copied mutex is structurally equal but
locking either side trips `-race`). Substitute
`cmp.Diff(want, got, cmpopts.EquateApproxTime(time.Millisecond),
cmpopts.IgnoreUnexported(sync.Mutex{}))`.

Goroutine leak detection: `go.uber.org/goleak` per package via a
`TestMain` that calls `goleak.VerifyTestMain(m)`. This catches the
specific bug from `02-current-audit.md` row §6.4 — the WebSocket
reader goroutine returning from a transport-close before the
send-channel is drained, which leaves the writer goroutine blocked
on `chan<- Envelope` send forever. Without `goleak`, the test
"passes" and the leak only surfaces hours later in production.

Fuzzing: stdlib `testing.F`. Two corpora: envelope parser and
agent-ref grammar (§7.5). Persisted under `testdata/fuzz/`.

CI reporting: `gotestsum` parses `go test -json` output into a
JUnit XML for the CI provider. Not a runtime import; not in
`go.mod`. Installed in CI via `go install gotest.tools/gotestsum`.

## Layered plan

### 1. Envelope layer

Scope: `messages/envelope_test.go`. The envelope is unchanged from
v1.0 (§5.1), so this layer exercises shape validation, registry
dispatch, and forward-compatibility.

What it tests:
- Marshal → unmarshal round-trip for every envelope type listed in
  `messages/types.go` (the full v1.0+v1.1 set: `session.hello`,
  `session.welcome`, `session.bye`, `session.error`, `session.ping`,
  `session.pong`, `session.ack`, `session.list_jobs`, `session.jobs`,
  `job.submit`, `job.accepted`, `job.event`, `job.result`,
  `job.error`, `job.cancel`, `job.subscribe`, `job.subscribed`,
  `job.unsubscribe`). Bug it catches: a payload struct tagged
  `json:"sessionId,omitempty"` instead of `json:"session_id"` —
  round-trip is lossy and the field silently drops.
- `testing.F` corpus seeded with one literal example per §13
  (heartbeat, ack/back-pressure, list_jobs+subscribe, lease expiry,
  budget, result_chunk, agent versioning — seven seeds) plus a
  generated set of malformed inputs (truncated JSON, nested
  payloads, unicode surrogates). Bug it catches: any panic on
  malformed JSON — every input must round-trip cleanly OR return
  an `*arcp.ProtocolError{Code: INVALID_REQUEST}`, never `panic`.
- Registry dispatch: `messages.ParseEnvelope` returns the right
  typed payload for known types. Bug it catches: a typo in the
  switch table mapping `"job.submit"` to the wrong concrete struct.
- Unknown types route to `UNIMPLEMENTED` (current SDK behavior per
  `02-current-audit.md`; restated for v1.1).
- Unknown top-level fields ignored per §5.1: an envelope with an
  extra `"x_future_field": 42` decodes successfully into the v1.1
  struct, with the extra field surfaced via the `envelopeWire`
  `extras` map (or dropped — Phase 4 picks). Bug it catches: a
  v1.0 client connecting to a v1.2 runtime erroring out on a new
  envelope-level field.

Special techniques: `testing.F` with seed corpus committed at
`testdata/fuzz/FuzzParseEnvelope/`.

### 2. Message layer

Scope: per-type payload validation, file-per-type in `messages/`.

What it tests:
- **Agent-ref grammar (§7.5)** via `testing.F` against
  `messages.ParseAgentRef`. Seeds: `"foo"`, `"foo@1.0.0"`,
  `"code-refactor@2.0.0-beta+sha.abc"`, `"Foo"` (must reject —
  uppercase), `"foo@"` (must reject — empty version), `"foo@bad/version"`
  (must reject — `/` outside version charset). Bug it catches:
  the parser accepting `"foo@@1"` because the split on `@` did not
  bound to one occurrence.
- **Budget grammar (§9.6)** `currency:decimal` via
  `messages.ParseBudgetAmount`. Cases: `"USD:1.00"` accepts;
  `"USD:1"` accepts; `"USD:1.000000001"` rejects (precision);
  `"usd:1.00"` rejects (currency must be ISO 4217 uppercase);
  `"USD:-1"` rejects (negative). Bug it catches: float-string
  conversion via `strconv.ParseFloat` silently quantizing
  `"1.0000000001"` to `1`.
- **`lease_constraints.expires_at` must be future (§9.5).**
  Inject `now` via a `clock.Clock` interface; test rejects
  `expires_at == now`, rejects `expires_at < now`, accepts
  `expires_at > now`. Bug it catches: a `>=` comparison that
  treats "now" as already expired so the very first
  `validateLeaseOp` after accept fails.
- **Event-body schemas per kind (§8.2).** One sub-test per kind:
  `log{level, message}`, `thought{text}`, `tool_call{tool, args,
  call_id}`, `tool_result{call_id, result|error}` (exactly one of
  the two — bug: accepting both), `status{phase, message?}`,
  `metric{name, value, unit?, dimensions?}`, `artifact_ref{uri,
  content_type, byte_size?, sha256?}`, `delegate` (§10), and the
  v1.1 additions `progress{current, total?, units?, message?}`
  (non-negative `current`, §8.2.1), `result_chunk{result_id,
  chunk_seq, data, encoding, more}` (§8.4).

### 3. FSM layer

Scope: `server/job_fsm_test.go`. Job lifecycle from §7.3:
`pending → running → {success | error | cancelled | timed_out}`.

What it tests:
- All 4 valid transitions out of `running` reach a terminal state.
- All forbidden transitions (`success → running`, `cancelled →
  success`, `pending → success` skipping `running`, etc.) panic
  in test mode and return `INTERNAL_ERROR` in production mode.
- Test-mode toggle: a package-level `var TestStrictFSM bool` set
  to `true` in `TestMain`. The FSM driver checks this and either
  panics (loud, fast) or returns an error (resilient). Bug it
  catches: a bad refactor that lets `cancelled` re-enter
  `running` because someone added `RequeueOnRetry` and forgot
  this verb is not in v1.1.

Special technique: `t.Run` per source-state, sub-`t.Run` per
target-state, with one `cmp.Diff` per pair against an oracle
table.

### 4. Lease layer

Scope: `internal/lease/*_test.go`. Glob, canonicalization, expiry,
budget counters.

What it tests:
- **Glob matching** anchored at both ends per §9.2: `*` matches
  exactly one path segment; `**` matches zero-or-more. Cases from
  the TS test `runtime/test/lease.test.ts:13-35`. Bug it catches:
  Go's `path/filepath.Match` accepting `*` as multi-segment on
  Windows path separators, which would silently expand
  `fs.read: ["/foo/*"]` to grant `/foo/bar/etc/passwd`.
- **Canonicalization** of targets before matching: `..` collapse,
  `.` strip, scheme lowercase. Test
  `validateLeaseOp({"fs.read": ["/safe/**"]}, "fs.read",
  "/safe/../etc/passwd")` rejects after canonicalization to
  `/etc/passwd`. Bug it catches: a path-traversal escape that
  bypasses lease enforcement.
- **`expires_at` enforcement with injected `now`.** A
  `clock.Mock`-style fake (see Phase 3 / 04-architecture) drives
  the test. Validate at `now < expires_at` → ok; advance fake to
  `now > expires_at`; validate → `LEASE_EXPIRED`. Bug it catches:
  the watchdog firing 100 ms before the deadline because it used
  `time.Now()` and the system clock skewed.
- **Budget counter atomicity.** Spin 64 goroutines each calling
  `validateLeaseOp` with a cost of `$0.50` against a `$1.00`
  budget; assert exactly 2 succeed, 62 fail with
  `BUDGET_EXHAUSTED`. Run under `-race`. Bug it catches: the
  classic "two `validateLeaseOp` calls both read remaining=$1.00,
  both subtract $0.50, both write $0.50, the budget is silently
  overspent by $0.50". This is precisely the bug a
  read-then-conditional-write pattern hides under low concurrency
  and only `-race` + a high-concurrency test surfaces.

### 5. Integration layer

Scope: `tests/integration/*_test.go`. A `Server` and a `Client`
paired over **both** `MemoryTransport` and `WebSocketTransport`
(loopback `127.0.0.1:0`, no TLS in tests, `httptest.Server` for the
upgrade). Each v1.1 feature gets one integration test, parameterised
across the two transports via a `t.Run(transportName, ...)` outer
loop. Every test uses `t.Context()` (Go 1.24+) — Phase 3 floor is
1.23 so use `ctx, cancel := context.WithCancel(context.Background());
t.Cleanup(cancel)` as the portable form.

- **heartbeat (§6.4).** Negotiate `heartbeat`, set
  `heartbeat_interval_sec=5`. Inject a `clock.Mock`; advance 11
  fake seconds with no inbound traffic; assert the transport
  closes with `HEARTBEAT_LOST`. Bug it catches: the watchdog
  using a real `time.Ticker` that is impossible to make
  deterministic, so the test would either wait 11 real seconds
  (slow) or flake on a busy CI runner (assertion races the
  ticker).
- **ack (§6.5).** Server emits 64 `job.event` envelopes; client
  has `AutoAck` with a 32-event coalesce window. Assert exactly
  2 `session.ack` envelopes arrive at the server, with
  `last_processed_seq=32` and `last_processed_seq=64`. Then drive
  100 events with the consumer paused (channel-buffered fake);
  assert a `job.event{kind:"status", body:{phase:"back_pressure"}}`
  arrives once the lag threshold is crossed. Bug it catches: a
  per-event ack flood (no coalesce) that drowns the server in
  acks.
- **list_jobs (§6.6).** Submit 5 jobs, page with `limit=2`,
  assert pages 1+2+3 yield 2+2+1 jobs, no duplicates, no missing
  ids, the third response has `cursor == nil`. Bug it catches:
  the opaque cursor embedding a slice index instead of a stable
  `(after_id, after_ts)` pair, so concurrent job-completion
  between pages shifts ids and yields a duplicate.
- **subscribe (§7.6).** Session A submits a job that emits 10
  events. Session B (same principal, different transport instance)
  calls `job.subscribe` with `history: true, from_event_seq: 0`.
  Assert B replays seqs 1..10 under B's own seq space, then
  receives live seqs. Then B calls `job.cancel` — assert
  `PERMISSION_DENIED`. Then A drops; assert B keeps observing.
  Bug it catches: replay leaking A's seq space into B's session,
  which would break B's `session.ack` highwater accounting.
- **lease_expires_at (§9.5).** Submit with `expires_at = now + 2s`.
  Advance the injected clock past 2.5s. Call a leased op; assert
  the op returns `LEASE_EXPIRED` and the job's terminal
  `job.error` carries the same code. Bug it catches: the
  watchdog firing but not propagating to in-flight tool calls,
  so the op succeeds against an expired lease.
- **cost.budget (§9.6).** Submit with `cost.budget: ["USD:1.00"]`.
  Emit three `metric` events: `cost.search:0.42`, `cost.fetch:0.70`
  (now remaining is -0.12), `cost.fetch:0.50`. Assert the third
  call returns `BUDGET_EXHAUSTED` surfaced as a `tool_result`
  body.error per §13.5. Bug it catches: applying the debit before
  the check (lets the last call partially succeed), or the check
  before the debit (off-by-one on the boundary value 0).
- **progress (§8.2.1).** Agent emits `progress{current:42,
  total:100, units:"files"}`. Client receives, asserts
  `current <= total`. Bug it catches: a negative-current emit
  not being rejected at the message-layer schema.
- **result_chunk (§8.4).** Agent streams 5 chunks of 1 KB each
  via `JobContext.StreamResult`. Client accumulates by
  `result_id`. Assert the assembled byte buffer is exactly 5 KB
  and matches input. Assert the terminating `job.result` carries
  `result_id` + `result_size=5120`. Bug it catches: mixing an
  inline `result` field on the terminating envelope with the
  chunks (§8.4 MUST NOT).
- **agent_versions (§7.5).** Three sub-cases against a server
  with `echo@1.0.0`, `echo@2.0.0`, default `echo→2.0.0`:
  (1) submit `echo` → resolves to `echo@2.0.0`;
  (2) submit `echo@1.0.0` → pinned;
  (3) submit `echo@3.0.0` → `AGENT_VERSION_NOT_AVAILABLE`.
  Bug it catches: default resolution executing the LATEST
  registered version instead of the configured default.

### 6. Conformance harness

Scope: `tests/conformance/*_test.go`. A machine-readable table
keyed on `CONFORMANCE.md` checkboxes — one row per requirement, one
sub-test per row. Each row carries `{section, requirement, status,
testFn}`. The harness runs `testFn`; if `testFn == nil` the row
status is `missing` and the harness fails. If `testFn` succeeds the
row status is `pass`.

Output: a JSON summary written to `tests/conformance/conformance.json`
with the shape `{section, requirement, status, duration_ms}`. The TS
suite emits a parallel `conformance.json`; a `scripts/diff-conformance.sh`
in CI diffs the two and fails if any spec row is implemented in one
SDK but missing in the other. This is how cross-language coverage
tracking stays honest — without the diff, the two SDKs drift.

## Coverage

Floor: **87% lines AND statements**. Canonical command:

```sh
go test -race -cover -coverpkg=./... -covermode=atomic -coverprofile=coverage.out ./...
```

`-covermode=atomic` is required because `-race` is on; the default
`set` mode is not race-safe.

Minimum to hit 87%:
- Cheap packages cover their public surface at 100%: `messages/`
  (envelope, payload structs), `arcp` root (errors, ids, version),
  `internal/version`, `internal/features`, `internal/lease`.
  These are pure-function packages; integration tests cover them
  for free.
- Expensive packages cover hot paths via integration tests:
  `server/` (handshake, FSM, dispatch), `client/` (handshake, auto-ack,
  reconnect), `transport/websocket/` (upgrade, read/write loop,
  close codes). Aim 80% for these; deltas are exercised via the
  conformance harness.
- Carve-out: `cmd/arcp/main.go` is tagged `//go:build !cover` and
  excluded from `-coverpkg`. CLI argument-parsing wiring is
  exercised by an end-to-end smoke test in
  `tests/smoke/cli_test.go` that invokes the built binary via
  `os/exec` — coverage is structurally unrecoverable for that
  path under `go test` and chasing it would mean stubbing
  `os.Exit`, which is worse than the alternative.

## Race detector + CI

All tests run under `-race`. The canonical command above already
includes it. CI never runs `go test` without `-race`.

CI Go matrix: **current Go (1.23)** + **previous minor (1.22)**.
Defence: 1.23 is the floor declared in `03-libraries.md` (needs
`iter.Seq` for §6.6 and §7.6); 1.22 is the immediately-previous
minor, included only to catch breaking changes in 1.23 that we
accidentally depended on. The 1.22 column may build-fail on
`iter.Seq` use, in which case it is removed and only 1.23 stays;
this is a deliberate signal, not a flake. Phase 3 sets the
authoritative floor; if it shifts to 1.24, drop 1.22.

CI platform matrix:
- `linux/amd64` — mandatory. Build, test, race, coverage.
- `darwin/arm64` — local-dev parity. Build + test under `-race`,
  no coverage (CI runner cost).
- `windows/amd64` — optional, advisory. `modernc.org/sqlite` is
  pure-Go and claims cross-platform parity including Windows
  (it is the reason that driver beat `mattn/go-sqlite3` in
  `02-current-audit.md`); a green Windows column corroborates
  the claim. If `coder/websocket` exhibits a Windows-only behaviour
  (it has not historically), the column documents it.

## Cancellation patterns under context

Every test that spawns a goroutine binds its lifetime to the
test. Two acceptable forms:

```go
// Go 1.24+: t.Context() returns a context canceled on cleanup.
ctx := t.Context()

// Portable to 1.23:
ctx, cancel := context.WithCancel(context.Background())
t.Cleanup(cancel)
```

No `time.Sleep` in tests. Flake mode: CI sleeps wake late (a 5 ms
`time.Sleep` on a loaded GitHub runner observed at 80 ms), so the
test assertion races the goroutine it intended to wait for —
sometimes the goroutine has progressed past the asserted state,
sometimes not. The fix is either inject a clock (preferred for
time-driven logic) or use a bounded `select`:

```go
select {
case got := <-ch:
    require.Equal(t, want, got)
case <-ctx.Done():
    t.Fatal(ctx.Err())
}
```

Goleak per package via `TestMain`:

```go
func TestMain(m *testing.M) {
    goleak.VerifyTestMain(m,
        goleak.IgnoreTopFunction("modernc.org/sqlite.(*conn).interrupt"),
    )
}
```

Top-functions safe to ignore (per package):
- `modernc.org/sqlite.(*conn).interrupt` — the SQLite driver's
  background interrupt goroutine outlives the test process and is
  acknowledged as such in its tracker.
- `go.opentelemetry.io/otel/sdk/trace.(*batchSpanProcessor).processQueue`
  — only if a test uses the OTel SDK (the library does not import
  it; the middleware package's tests might).

Every other top-function leaking is a real bug, surfaced before
production.

## Closing — ship blockers

Tests that must pass before tagging v1.1:

- Round-trip of every example envelope in §13 of
  `draft-arcp-02.1.md` (`messages/envelope_test.go` `TestSpec13Examples`).
- Conformance harness shows zero `missing` rows
  (`tests/conformance/conformance.json` has no `status: "missing"`).
- Coverage ≥ 87% on lines and statements via the canonical
  `go test -race -cover -coverpkg=./... -covermode=atomic ./...`.
- `-race` clean across all platforms in the CI matrix.
- `goleak.VerifyTestMain` clean per package; no top-function
  ignore added during this release cycle without a tracker link.
