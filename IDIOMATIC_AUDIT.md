# Idiomatic Go Audit — `arcp-go`

Scope: `/Users/nficano/code/arpc/go-sdk` (module `github.com/fizzpop/arcp-go`,
Go 1.25.0). Generated working from `main` at commit `e8c2258`.

## Phase 1 — Baseline

### Toolchain & layout
- Toolchain: `go1.25.5 darwin/arm64`. Module declares `go 1.25.0`.
- Existing config: [.golangci.yml](.golangci.yml) (v2), [Makefile](Makefile)
  exposes `fmt`, `vet`, `lint`, `test`, `cover`, `build`, `gates`.
- Generated files: none. No `// Code generated ... DO NOT EDIT.` markers.
- Vendor directory: none.

### Package inventory

| Path | Files | Responsibility |
| --- | --- | --- |
| `.` (`arcp`) | doc, version, envelope, errors, extensions, ids, trace | Canonical envelope, typed ids, error model, extension namespace rules, trace context. |
| `auth` | doc, auth, bearer | `Verifier` interface + `AnonymousVerifier`, `BearerVerifier`, `MultiVerifier`. |
| `client` | doc, client | `Client` wrapping a session; `Open`, `Send`, `Recv`, `Ping`, `Close`. |
| `cmd/arcp` | main | Placeholder CLI; subcommands deferred to Phase 7. |
| `messages` | doc, common, session, control, execution, streaming, artifacts, human, permissions, subscriptions, telemetry | Typed payload structs + `init()` registration. |
| `runtime` | doc, runtime | Server-side `Runtime`; handshake + run loop. |
| `store` | doc, eventlog, schema.sql | SQLite-backed append-only event log + replay. |
| `transport` | doc, transport, memory | `Transport` interface + in-memory paired transport. |
| `internal/ulid` | doc, ulid | Monotonic ULID generator wrapping `oklog/ulid`. |
| `internal/deadline` | doc | **Empty package** — only a doc.go. Placeholder. |
| `examples` | — | **Empty directory.** Placeholder for Phase 7. |
| `tests` | handshake_test, helpers_test | Cross-package integration tests in `package arcptest_test`. |
| `testdata/golden` | ping.json | Wire-format snapshot. |

### Baseline tooling
All gates pass on `main`:

| Tool | Result |
| --- | --- |
| `go build ./...` | exit 0 |
| `go vet ./...` | exit 0 |
| `gofmt -l .` | empty |
| `goimports -l .` | empty (installed at `$(go env GOPATH)/bin/goimports`) |
| `golangci-lint run` (project config) | `0 issues` |
| `golangci-lint run` (strict config: errcheck, errorlint, govet, staticcheck, revive, gocritic, unused, ineffassign, misspell, unconvert, nilerr, prealloc + gofmt) | `0 issues` |
| `go test ./... -race -count=1` | all packages pass |

The codebase is already well past the basic-hygiene bar. The findings below are
things linters do not catch: idiomatic naming, dead code, and small structural
choices.

---

## Phase 2 — Findings, by category

References use `path:line` form to make changes auditable.

### Naming

1. **`auth.MultiVerifier.BySchema` is misnamed.**
   [auth/auth.go:46](auth/auth.go#L46) — the field holds a
   `map[messages.AuthScheme]Verifier`. The key type is *`AuthScheme`*; the
   field calls itself "schema" (a different word). All callers use
   `BySchema:` ([tests/handshake_test.go:109](tests/handshake_test.go#L109),
   [tests/helpers_test.go:60](tests/helpers_test.go#L60)).
   Rename to `ByScheme` for consistency with the key type. **Breaking** —
   public field. No external callers known yet (all internal to repo).

2. **Truncated unit suffixes break the no-Hungarian rule.**
   The wire JSON tags spell out the unit (`_seconds`, `_ms`); the Go field
   names truncate to a single letter or short abbreviation and read as
   Hungarian:
   - [messages/common.go:78](messages/common.go#L78) `ArtifactRetentionDefaultS`
   - [messages/common.go:79](messages/common.go#L79) `ArtifactRetentionMaxS`
   - [messages/control.go:61](messages/control.go#L61) `Cancel.DeadlineMS`
   - [messages/execution.go:105](messages/execution.go#L105) `JobHeartbeat.DeadlineMS`
   - [messages/execution.go:154](messages/execution.go#L154) `JobScheduleWhen.AfterSec` (json `after`, comment says "seconds")

   Spell the unit in full (`ArtifactRetentionDefaultSeconds`,
   `DeadlineMilliseconds`, `AfterSeconds`). Wire tags stay the same. **Breaking**
   for exported fields.

3. **`Sha256` should be `SHA256` per the acronym rule.**
   `SHA` is an acronym; Go style keeps acronyms in their case (`URL`, `ID`,
   `HTTP`, `SHA`).
   - [messages/streaming.go:60](messages/streaming.go#L60) `StreamChunk.Sha256`
   - [messages/artifacts.go:24](messages/artifacts.go#L24) `ArtifactRef.Sha256`
   - [messages/artifacts.go:38](messages/artifacts.go#L38) `ArtifactPut.Sha256`

   **Breaking** — exported fields. Tested via golden file.

4. **`messages.Capabilities.HeartbeatRecovery` is `string` with documented
   enum values in a trailing comment.**
   [messages/common.go:76](messages/common.go#L76):
   `HeartbeatRecovery string` with `// "fail" | "block"`. Other enum-shaped
   fields in the same file declare a named string type with `Defined …`
   constants (`AuthScheme`, `TrustLevel`, `Priority`, `LogLevel`,
   `CancelTarget`). Same for `BinaryEncoding []string` (line 77) — it is a
   restricted set per RFC §7. Either declare types and constants, or accept
   a documented inconsistency.

### Errors

5. **String-matching SQLite uniqueness errors.**
   [store/eventlog.go:236-247](store/eventlog.go#L236-L247) classifies a
   `UNIQUE` constraint violation by substring scan. The `errorlint` rule
   normally forbids this; the comment justifies it because
   `modernc.org/sqlite` does not export a typed sentinel. Worth noting as a
   known idiomatic exception. If the driver gains an exported error type,
   migrate to `errors.As`. No change needed now — leave the comment in
   place.

6. **Best-effort sends discard their error without surface logging.**
   [runtime/runtime.go:97](runtime/runtime.go#L97),
   [runtime/runtime.go:105](runtime/runtime.go#L105),
   [runtime/runtime.go:113](runtime/runtime.go#L113),
   [runtime/runtime.go:118](runtime/runtime.go#L118),
   [client/client.go:124](client/client.go#L124).
   `_ = r.sendReject(...)` (etc.) silently drops a transport error on the
   teardown path. Idiomatic Go either logs at the boundary or returns —
   here neither happens. Recommend a `r.opts.Logger.Warn(...)` on the
   discarded error so operators can see broken-transport conditions during
   teardown. Behaviour-preserving change.

### Interfaces

7. **`MessageType` interface is at the producer.**
   [envelope.go:14-16](envelope.go#L14-L16). Implementations live in
   `messages/`, which imports `arcp` to register. The interface is consumed
   by `Envelope` decoding inside `arcp` itself, so this *is* consumer-side —
   `arcp` is the consumer of "anything that can name itself". OK.

8. **`auth.Verifier`** at [auth/auth.go:22-24](auth/auth.go#L22-L24) and
   **`transport.Transport`** at [transport/transport.go:18-30](transport/transport.go#L18-L30)
   are defined in producer packages with multiple implementations and
   external use. Idiomatic.

   No changes needed for interfaces.

### Concurrency

9. **`sync.Once` shared via pointer between paired transports.**
   [transport/memory.go:23](transport/memory.go#L23),
   [transport/memory.go:32-36](transport/memory.go#L32-L36).
   Two `memoryTransport` ends share `closeOnce *sync.Once` so either
   end closing terminates both. Correct (pointer is required to share
   state); the `// Either end's Close terminates both sides` comment in
   `NewInMemoryPair` covers the why. No change.

10. **Context not stored in structs.** Verified by grep: no struct fields
    of type `context.Context`. Every blocking call takes `ctx` as the first
    parameter. Good.

11. **Goroutine ownership.** The only goroutine in non-test code is the
    `Runtime.Serve` callsite — and that is owned by *callers* (Serve is a
    blocking function). Tests in [tests/helpers_test.go:47-52](tests/helpers_test.go#L47-L52)
    spawn a goroutine and clean it up via `t.Cleanup`. Good.

### Package layout

12. **Empty `internal/deadline/` package.**
    [internal/deadline/doc.go](internal/deadline/doc.go) is the only file
    and contains a placeholder docstring. Either implement (Phase 3+
    work) or remove the directory until it has a real file. Empty packages
    are non-idiomatic and confuse `go list ./...` consumers.

13. **Empty `examples/` directory.** Same issue. Either add a stub program
    that compiles or remove until Phase 7.

14. **`tests/` directory with only `*_test.go`.** Slightly non-idiomatic —
    Go normally co-locates tests with the code they exercise. The current
    layout is reasonable for cross-package integration tests
    (`arcp` + `client` + `runtime` + `transport` + `messages` all
    together) and `package arcptest_test` makes the boundary clear. No
    change needed; flag for future consideration if these tests become
    package-specific.

### Style

15. **Hand-rolled `sortStrings` insertion sort.**
    [extensions.go:117-126](extensions.go#L117-L126). Comment claims
    "to avoid pulling in sort just for short slices". `sort` is in the
    standard library — no dependency cost. Go 1.21+ provides
    [`slices.Sort`](https://pkg.go.dev/slices#Sort), which is what
    idiomatic Go 1.25 code uses. Two callers
    ([envelope.go:317](envelope.go#L317),
    [extensions.go:113](extensions.go#L113)). Replace with
    `slices.Sort(out)` and delete `sortStrings`. Behaviour-preserving.

16. **Dead "noop for unused" block.**
    [messages/common.go:168-172](messages/common.go#L168-L172):
    ```go
    var _ = json.RawMessage(nil)
    var _ = time.Time{}
    var _ = rawJSONOrNil
    ```
    Comment claims this keeps the unused-import linter satisfied "for
    files that pull in json/time only via reflection." Both `json` and
    `time` are *directly* used by other declarations in the file
    (`json.RawMessage` in `Capabilities.BinaryEncoding`/`Extensions`,
    `time.Time` in `Lease.ExpiresAt`). The block is dead. Remove.

17. **`for i := 0; i < N; i++` loops in tests.** Go 1.22+ supports
    `for i := range N`. Module is on Go 1.25 so this is available.
    Occurrences:
    - [ids_test.go:33](ids_test.go#L33)
    - [internal/ulid/ulid_test.go:14,33,49](internal/ulid/ulid_test.go#L14)
    - [store/eventlog_test.go:62,112,142,183,345](store/eventlog_test.go#L62)

    Pure style. Flag, do not require — both forms remain idiomatic.

18. **Loop-variable shadow no longer needed in Go 1.22+.**
    [messages/registry_test.go:17](messages/registry_test.go#L17):
    `typeName := typeName`. Go 1.22 made loop variables per-iteration; the
    shadow is a no-op. Remove.

19. **Comments on exported identifiers.** Spot-checked across all
    `*.go` files; every exported type, func, and var has a comment that
    leads with the identifier name and reads as a complete sentence.
    Conforms to the rule.

20. **`init()` use in `messages/`.** Each `messages/*.go` registers its
    types via `init()`. The package doc and design rules
    ([envelope.go:271-273](envelope.go#L271-L273)) explicitly carve out
    this exception. Acceptable.

21. **Constructors that just set zero values.** Spot-checked: no
    constructors of this kind. `NewExtensionRegistry` does set a non-nil
    map (defensible — `Add` lazy-init guards anyway, but starting non-nil
    is clearer); see also fields like `messageRegistry` initialized at
    declaration. OK.

22. **Receiver name consistency.** Verified: every type uses the same
    short receiver across all its methods (`e *Envelope`, `e *Error`,
    `r *Runtime`, `c *Client`, `l *EventLog`, `g *Generator`,
    `t *memoryTransport`, `r *ExtensionRegistry`, `m *MultiVerifier`,
    `v *BearerVerifier`).

### Tests

23. **Black-box vs. white-box mix is correct.** All `*_test.go` for the
    root `arcp` package use `package arcp_test`; `store` has both
    [store/eventlog_test.go](store/eventlog_test.go) (`store_test`) and
    [store/internal_test.go](store/internal_test.go) (`store`, with a
    leading comment explaining it tests internals). Idiomatic.

24. **`t.Helper`, `t.Cleanup`, `t.Run`, `t.Parallel` usage.** Verified
    correct usage across the suite. No `assert`-style libraries.

### Generics

- No generic functions or types in the codebase. Nothing to flag.

---

## Bugs found during refactor (not in scope)

None observed. All tests pass with `-race`; the audit pass is style-only.

---

## Recommended commit plan (Phase 2 onward)

Each is its own commit, in this order. None changes wire format. Items 1, 2,
3 are breaking at the Go API level; 5 is best-effort additive logging; the
rest are pure cleanup.

1. **`style(arcp): replace hand-rolled sortStrings with slices.Sort`**
   — finding 15. Pure cleanup, no API change.

2. **`style(messages): drop dead noop-for-unused block`**
   — finding 16. Pure deletion.

3. **`style(*): drop redundant Go-1.22 loop-var shadow`**
   — finding 18. One-line removal in registry_test.go.

4. **`naming(auth): rename MultiVerifier.BySchema to ByScheme`** —
   finding 1. **BREAKING**: public field rename. Mechanical.

5. **`naming(messages): spell unit suffixes in full`** — finding 2.
   Renames `*S`/`*MS`/`*Sec` fields to `*Seconds`/`*Milliseconds`. JSON
   tags unchanged; behaviour preserved. **BREAKING**: public field renames.

6. **`naming(messages): SHA256 acronym casing`** — finding 3. Renames
   `Sha256` to `SHA256` on three structs. **BREAKING**: public field rename.

7. **`errors(runtime,client): log discarded transport-teardown errors`** —
   finding 6. Adds best-effort `Logger.Warn` calls; behaviour preserved.

Items 4-6 cluster the breaking changes. If external callers are rumored,
collapse 4-6 into a single "v0.2 naming sweep" commit so consumers see one
break.

Items deliberately deferred:

- Empty `internal/deadline/` and `examples/` (findings 12, 13). Owners
  state these are placeholders for upcoming phases — leave alone, revisit
  before Phase 7.
- `HeartbeatRecovery`/`BinaryEncoding` typing (finding 4). The
  string-with-comment form is RFC-faithful; promoting to a typed enum is a
  judgment call that warrants RFC discussion.
- Pre-1.22 `for i := 0` loop forms (finding 17). Both forms remain
  idiomatic; mass-rewriting to `for i := range N` would touch many files
  for marginal benefit.

---

**Stop point.** Awaiting review of this audit before any code changes.
