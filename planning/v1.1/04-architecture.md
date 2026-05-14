# 04 — Architecture: ARCP v1.1 Go SDK

Module layout, type model, concurrency model. No Go code is written
in this pass; signatures only where the public surface needs pinning.
This document is design intent — `03-spec-delta.md` is the wire
truth, `02-current-audit.md` is the inventory of what survives the
rewrite.

## Module structure

Single module, path `github.com/agentruntimecontrolprotocol/go-sdk`
(already in `go.mod:1`). Keep it. The path matches the spec
authority's namespace, satisfies `go install` for `cmd/arcp`, and
mirrors the TS scope `@arcp/*`. Two-segment vendored paths (e.g.
`github.com/arpc/go-sdk`) save typing but lose the protocol-org
prefix that makes the import line self-documenting on a third-party
file.

Sub-package layout (additive to the surviving scaffolding from
`02-current-audit.md`):

| Package | Surface | Notes |
| --- | --- | --- |
| `arcp/` (module root) | `Envelope`, `MessageType` interface, error sentinels, feature constants, `Code`, `IsRetryable` | Root package, no transport or session logic. Imported by every other package. Mirrors TS `@arcp/core` envelope+errors+brands. |
| `arcp/messages/` | Typed payload structs: `SessionHello`, `SessionWelcome`, `JobSubmit`, `JobAccepted`, `JobEvent`, `JobResult`, `JobError`, `JobCancel`, `SessionPing`, `SessionPong`, `SessionAck`, `SessionListJobs`, `SessionJobs`, `JobSubscribe`, `JobSubscribed`, `JobUnsubscribe` | Each file registers its `MessageType`s in `init()`. No behaviour. Mirrors TS `packages/core/src/messages/`. |
| `arcp/transport/` | `Transport` interface (already in `transport/transport.go:18`), `MemoryTransport`, `WebSocketTransport`, `StdioTransport` | Interface stays. Memory keeps semantics, gains the new envelope. WebSocket and stdio are new. |
| `arcp/client/` | `Client`, `JobHandle`, `Subscription`, `ResultAssembler` | Mirrors TS `packages/client/src/client.ts`. Consumes `transport.Transport`. |
| `arcp/server/` | `Server`, `Agent`, `AgentFunc`, `Session`, `Job`, `JobContext`, `ResultWriter` | Mirrors TS `packages/runtime/src/{server,job,lease}.ts`. The merge of TS `core` (transport/messages) into Go's root + `messages/`, and the merge of TS `runtime` + the runtime parts of `sdk` into Go's `server/`, is justified below. |
| `arcp/auth/` | `Verifier` interface, `BearerVerifier` | Survives from `auth/bearer.go`. Drop `AnonymousVerifier` and `MultiVerifier` (`02-current-audit.md` "Anti-salvage list" §"auth"). |
| `arcp/middleware/{nethttp,chi,otel}` | WebSocket upgrade handlers for `net/http` and `chi`; OTel attribute adapter | Phase 5 refines. Equivalents of TS `packages/middleware/{node,express,fastify,hono,otel}`. |
| `arcp/cmd/arcp/` | CLI binary | Equivalent of TS `@arcp/sdk` CLI; `cmd/arcp/` directory already exists. |
| `arcp/internal/lease/` | Lease validation, budget counters, expiry watchdog | Not API. Imported only by `server/`. |
| `arcp/internal/eventlog/` | Event log over SQLite (`modernc.org/sqlite`, kept per `02-current-audit.md` "Salvageable assets") | Not API. |
| `arcp/internal/version/` | `ARCPVersion = "1"` constant; `V1_1_FEATURES` list mirroring TS `packages/core/src/version.ts` | Not API. |
| `arcp/internal/features/` | `Intersect(a, b []string) []string` per `01-spec-delta.md` "Capability negotiation table" | Not API. |
| `arcp/internal/idstore/` | `(principal, idempotency_key) → job_id` dedupe map per §7.2 | Not API. |

**TS-to-Go package collapse.** TS splits `core` / `client` /
`runtime` / `sdk`. Go's compilation unit is the package; four
packages where two suffice means four `import` lines per call site
and four `go.mod` lines per consumer if they were modules.

- TS `@arcp/core` (envelope, messages, transport, store, auth) maps
  to Go root `arcp/` + `arcp/messages/` + `arcp/transport/` +
  `arcp/internal/eventlog/` + `arcp/auth/`. Keeping `messages/` and
  `transport/` separate from the root is Go-specific: the root
  package must be importable without dragging in `database/sql` and
  `net/http` for tiny consumers (e.g. a process that only needs
  `arcp.Envelope` to forward bytes).
- TS `@arcp/runtime` (server, job, lease) maps to Go `arcp/server/`
  with `internal/lease` and `internal/eventlog` underneath. There
  is no Go idiom rewarding a separate `lease/` package when its
  only consumer is `server/`; the rule "interface defined where
  consumed" applies — `LeaseValidator` belongs in `server/`, not
  exposed as public API.
- TS `@arcp/sdk` (re-exports + CLI) maps to Go `arcp/cmd/arcp/`
  plus the absence of a re-export package: in Go, callers import
  `arcp/client`, `arcp/server`, and `arcp/transport` directly.
  `@arcp/sdk`-as-umbrella doesn't carry weight in Go.
- TS `@arcp/client` maps 1:1 to `arcp/client/`.

## Concurrency model

**Hard rule: every public method's first argument is
`context.Context`.** This includes `Transport.Send/Recv` (already
true at `transport/transport.go:21,25`), `Client.Submit`,
`Server.Accept`, `Server.Close`. Methods that return a derived
handle (`JobHandle`, `Subscription`) embed a child context whose
cancellation propagates to the underlying read loop.

**Subscribe API: `<-chan Event`, not `iter.Seq2`.** Go 1.23's
`iter.Seq2[Event, error]` is more ergonomic at the call site, but
the subscription's lifecycle is server-driven (it ends when the job
terminates or the subscriber unsubscribes), not consumer-driven
(`break` out of `for range`). The channel form expresses "the
producer closes when there are no more events" precisely; the
iterator form pushes the burden onto the user to remember to
`return` from the yield function. Choose `<-chan Event` and pair it
with a `Subscription.Err() error` accessor (a TS-`AsyncIterator`-
style protocol won't survive the goroutine-leak audit).

The Go-specific hazard, named: **goroutine leak if the consumer
stops reading.** Channel ownership: the `Subscription` owns the
channel and closes it on terminal event or transport close. The
consumer MUST NOT close it (panic on send-to-closed). If the
consumer drops the subscription without calling `Subscription.Close`,
the server-side fan-out goroutine blocks on send forever — so every
fan-out send goes through a `select { case ch <- evt: case <-sub.ctx.Done(): return }`
pattern, and `Subscription.Close` cancels `sub.ctx`. This is the
single subtlest concurrency point in the SDK; `02-current-audit.md`
flagged §7.6 fan-out as risk H for exactly this reason.

**Heartbeat lifecycle.** One `*time.Ticker` per session per
direction (inbound watchdog + outbound emitter). Shutdown order is
load-bearing:

1. Cancel `session.ctx` (which `Accept`/`Connect` derived).
2. Wait for the ticker goroutine to return (`<-ticker.done`).
3. Drain the writer's send queue.
4. Close the transport.

Reverse this — close the transport first — and the ticker
goroutine's next `Send(ctx, ping)` writes to a closed transport,
producing a spurious `transport: closed` error and (worse) a panic
on `close()`-twice if the write goroutine ALSO calls `Close`. The
deadlock to name: ticker goroutine holding the writer mutex while
`Server.Close` blocks waiting to acquire the same mutex to send a
`session.bye` — fixed by `session.ctx` cancellation flushing the
writer first.

**Read loop / write loop.** One reader goroutine per transport
(already documented at `transport/transport.go:17` "each end is
expected to be driven by a single reader and a single writer"). For
the writer, two designs:

- **Mutex-guarded `Send`.** Each caller acquires `session.writeMu`,
  marshals, writes, releases. Simple. Reduces to TS's
  promise-serialized writes via the mutex. Hazard: a slow write
  blocks every other writer (heartbeat ticker, event emitter,
  cancel handler). The mutex never deadlocks because no caller
  holds another lock across `Send`.
- **Dedicated writer goroutine + bounded channel.** Every producer
  sends `Envelope` on `session.outbox`; one goroutine drains and
  writes. Hazard: producers block on a full channel (back-pressure
  on the agent), but recovery is uniform — `ctx.Done()` unblocks
  every producer's `select`. Channel ownership: `Server` owns
  `outbox`, closes it after the writer drains.

Pick the **dedicated writer goroutine**. Justification: it makes
`session.ack` flow control (§6.5) a single point of policy
("decrement available outbox space before sending"), it isolates
the heartbeat ticker from contention (the ticker never holds a
lock), and it gives the `BACK_PRESSURE` status emission (§6.5,
back_pressure phase) a natural trigger — outbox length crossing a
threshold. TS's promise-serialized writes don't translate because
TS has no preemption: a Go writer holding a mutex through a slow
syscall is a different shape of problem than a JS Promise chain.

**Cancellation propagation for jobs.** `JobContext.ctx` is derived
from the session context with an additional `context.WithCancel`
keyed off the job lifecycle. Terminal events (`job.result`,
`job.error`, or a `job.cancel` from the submitting session) cancel
this context. The agent function returning, panicking (recovered
with `defer recover()` at the server boundary; see §"Hard rules"
below), or the session dying all collapse to the same cancellation.
Client-side, `JobHandle.Done() <-chan struct{}` mirrors v1.0's
promise: closed when the job reaches a terminal event. `Wait(ctx)`
returns `(*Result, error)` and respects `ctx.Done()` — the v1.0
spec requires no specific shape here, but the channel-plus-Wait
pair is the Go idiom (compare `cmd/go`'s `os/exec.Cmd.Wait`).

## Type model

**Envelope.** One struct, per §5.1:

```
type Envelope struct {
    ARCP      string          // always "1"
    ID        string          // ULID or UUIDv7
    Type      string          // e.g. "job.submit"
    SessionID string          // empty on session.hello
    JobID     string          // empty unless job-scoped
    TraceID   string          // 32 hex chars when present
    EventSeq  uint64          // 0 unless this is a job.event/job.result/job.error
    Payload   json.RawMessage // typed via MessageType registry
}
```

`json.RawMessage` lets the read loop hand off envelopes to the
correct handler without parsing the payload twice. Unknown
top-level fields are ignored at the decoder layer per §5.1
forward-compat, which `02-current-audit.md` row "§5.1 unknown-field
passthrough" flagged as currently broken in `envelope.go:88-108`.

**MessageType registry.** Keep the `MessageType` interface from
the current SDK (`messages/` survives in shape, dies in content per
`02-current-audit.md` table row "messages/"):

```
type MessageType interface { ARCPType() string }
```

Each `messages/*.go` file registers its types in `init()`:

```
func init() {
    arcp.RegisterMessageType(&SessionHello{})
    arcp.RegisterMessageType(&SessionWelcome{})
    ...
}
```

**Justification for `init()` side effects here, despite the no-side-
effects hard rule below:** the alternative is a single
`messages/register.go` with one giant function called from `main`,
which (a) couples every consumer to import the registration code
explicitly and (b) duplicates the wire-type string outside the
struct that owns it. The Go idiom "interface defined where consumed"
applies in reverse: the registration belongs next to the struct
because the struct's `ARCPType()` method is the registration key.
This is the same pattern `image` uses for `image.RegisterFormat` and
`database/sql` uses for driver registration. Document the exception
explicitly in package doc.

**`iota` for wire types?** No. Wire types are spec-stable strings
(`"job.submit"`, `"session.hello"`). An `iota` enum buys nothing
and complicates JSON round-tripping. Where `iota` does fit: the
**job lifecycle state machine** (`JobStatus`: `Pending`, `Running`,
`Success`, `Error`, `Cancelled`, `TimedOut`), where the values are
internal-only and a `String()` method (hand-written switch) maps to
the spec strings on the wire. No code generation for v1.1.

**Lease.** Keyed by typed capability string:

```
type Capability string

const (
    CapFSRead       Capability = "fs.read"
    CapFSWrite      Capability = "fs.write"
    CapNetFetch     Capability = "net.fetch"
    CapToolCall     Capability = "tool.call"
    CapAgentDelegate Capability = "agent.delegate"
    CapCostBudget   Capability = "cost.budget"
)

type Lease map[Capability][]string
```

`Capability` is `string`-typed to keep JSON encoding trivial and to
admit vendor extensions (any string matching the §9.2 grammar). The
constants enumerate §9.2's reserved namespaces. The pattern strings
are exactly as the spec defines them — globs for `fs.*`, URL globs
for `net.fetch`, tool/agent name globs for `tool.call` and
`agent.delegate`, amount strings (`"USD:5.00"`) for `cost.budget`.
Budget counters are mutable per-job runtime state and live on
`server/Job`, **not** on the immutable `Lease`. The spec is explicit
at §9.6: "Each is a separate counter, initialized at the budgeted
value at job acceptance" — counters are derived from the lease, not
part of it.

**Where `any` is allowed.** Only two places: (1) unknown envelope
payloads passed through to user code via the `x-vendor.*` extension
mechanism (§15), where the user is the only one who knows the type;
(2) the `*JobContext.Metric(name string, value float64, unit string, dims map[string]string)`
dimensions value — but dims is `map[string]string`, not `map[string]any`,
so even that escapes the rule. The `AgentFunc` signature uses
`json.RawMessage` for input and `(any, error)` for output: the
output is `any` because the agent decides its result shape and the
server marshals it. Forbidden by hard rule: `any` as a public method
return type. `(any, error)` from `AgentFunc` is a callback shape,
not a return type from an SDK method.

**No `interface{}` (or `any`) returned from public SDK methods.**
`Client.Submit` returns `(*JobHandle, error)`, not `(any, error)`.
`JobHandle.Wait` returns `(*Result, error)` where `Result.Output`
is `json.RawMessage` (the caller `Unmarshal`s into its known type).
Compare TS `Job<T>` generics — Go's generic story for return-typed
results is `func[T any](ctx, *JobHandle) (T, error)`, which the
package may offer as a `client.WaitTyped[T]` helper but does not
take as the default.

## Errors

**Sentinel `*arcp.Error` per spec code, declared in `errors.go`.**
The current SDK's shape (`errors.go:14-37`) is close but its
**code names are wrong** (gRPC-style, per `02-current-audit.md`
table row "Error codes"). Replace wholesale with the 12 v1.0 + 3
v1.1 codes from §12.

```
type ErrorCode string

const (
    CodePermissionDenied         ErrorCode = "PERMISSION_DENIED"
    CodeLeaseSubsetViolation     ErrorCode = "LEASE_SUBSET_VIOLATION"
    CodeJobNotFound              ErrorCode = "JOB_NOT_FOUND"
    CodeDuplicateKey             ErrorCode = "DUPLICATE_KEY"
    CodeAgentNotAvailable        ErrorCode = "AGENT_NOT_AVAILABLE"
    CodeAgentVersionNotAvailable ErrorCode = "AGENT_VERSION_NOT_AVAILABLE" // new in 1.1
    CodeCancelled                ErrorCode = "CANCELLED"
    CodeTimeout                  ErrorCode = "TIMEOUT"
    CodeResumeWindowExpired      ErrorCode = "RESUME_WINDOW_EXPIRED"
    CodeHeartbeatLost            ErrorCode = "HEARTBEAT_LOST"
    CodeLeaseExpired             ErrorCode = "LEASE_EXPIRED"                // new in 1.1
    CodeBudgetExhausted          ErrorCode = "BUDGET_EXHAUSTED"             // new in 1.1
    CodeInvalidRequest           ErrorCode = "INVALID_REQUEST"
    CodeUnauthenticated          ErrorCode = "UNAUTHENTICATED"
    CodeInternalError            ErrorCode = "INTERNAL_ERROR"
)

type Error struct {
    Code      ErrorCode
    Message   string
    Retryable bool
    Details   map[string]any
    cause     error
}

func (e *Error) Error() string         { ... }
func (e *Error) Unwrap() error         { return e.cause }
func (e *Error) Is(target error) bool  { ... } // matches by Code
```

`*Error` wraps a cause via the `Unwrap`/`%w` chain — `errors.Is` and
`errors.As` work across the sentinel layer (already true in
`errors.go`; preserve). Public helper:

```
func Code(err error) ErrorCode  // walks the chain; returns CodeInternalError on miss
func IsRetryable(err error) bool
```

**Raise sites for the three v1.1 codes** (per `01-spec-delta.md`
"New error codes" table):

- `CodeAgentVersionNotAvailable` — `server.handleJobSubmit`, after
  `agent.ParseRef` resolves a pinned version not in the inventory.
  Returned via `session.error` (not `job.error` — the job never got
  a `job_id`).
- `CodeLeaseExpired` — `internal/lease.ValidateOp` when called
  past `expires_at`; also raised by the expiry watchdog
  (`internal/lease.watchdog`) firing while a job runs. Surfaces as
  `tool_result` body.error preferentially per §9.5, falling back to
  `job.error` when the runtime preempts.
- `CodeBudgetExhausted` — `internal/lease.ValidateOp` when any
  budget counter is `≤ 0`. Surfaces preferentially as
  `tool_result` body.error per §9.6 ("Runtimes SHOULD prefer the
  `tool_result` form").

All three are `Retryable: false` per §12 ("`retryable: false`
matters for the Go client: `arcp.IsRetryable(err)` returns `false`
against these three sentinels").

## Public API sketch

Signatures only. Bodies are out of scope.

### Root package

```
type Envelope struct { ... }                       // §Type model
type MessageType interface { ARCPType() string }
type Capability string
type Lease map[Capability][]string
type Currency string
type ErrorCode string
type Error struct { ... }

func RegisterMessageType(m MessageType)            // init-time only; panics on duplicate
func Code(err error) ErrorCode
func IsRetryable(err error) bool
func NewID() string                                // ULID; alias for backwards compat
```

### `transport/`

```
type Transport interface {
    Send(ctx context.Context, env arcp.Envelope) error
    Recv(ctx context.Context) (arcp.Envelope, error)
    Close() error
}

func NewMemoryPair() (Transport, Transport)
func DialWebSocket(ctx context.Context, url string, opts WebSocketOptions) (Transport, error)
func NewStdioTransport(in io.Reader, out io.Writer) Transport
```

(Already defined at `transport/transport.go:18`; envelope type
underneath changes, interface does not.)

### `client/`

```
type Client struct { ... }

func NewClient(opts Options) (*Client, error)
func (c *Client) Connect(ctx context.Context, t transport.Transport) (*Welcome, error)
func (c *Client) Welcome() *Welcome                     // accessor; no Get prefix
func (c *Client) HasFeature(name string) bool
func (c *Client) Submit(ctx context.Context, req SubmitRequest) (*JobHandle, error)
func (c *Client) Ack(ctx context.Context, seq uint64) error
func (c *Client) ListJobs(ctx context.Context, req ListJobsRequest) (*JobList, error)
func (c *Client) Subscribe(ctx context.Context, jobID string, opts SubscribeOptions) (*Subscription, error)
func (c *Client) Close(ctx context.Context) error

type JobHandle struct { ... }
func (h *JobHandle) ID() string
func (h *JobHandle) Done() <-chan struct{}
func (h *JobHandle) Wait(ctx context.Context) (*Result, error)
func (h *JobHandle) Cancel(ctx context.Context, reason string) error
func (h *JobHandle) Events() <-chan Event              // closed at terminal
func (h *JobHandle) Chunks() <-chan ResultChunk        // closed at terminal; nil if not streaming
func (h *JobHandle) Err() error                        // populated after Done closes

type Subscription struct { ... }
func (s *Subscription) JobID() string
func (s *Subscription) Events() <-chan Event
func (s *Subscription) Close(ctx context.Context) error
func (s *Subscription) Err() error
```

`Ack`/`ListJobs`/`Subscribe` return `*Error{Code: CodeInvalidRequest}`
if the feature is not in the negotiated intersection
(`01-spec-delta.md` "Capability negotiation table" closing
paragraph).

### `server/`

```
type Server struct { ... }

func NewServer(opts Options) (*Server, error)
func (s *Server) RegisterAgent(name string, fn AgentFunc)
func (s *Server) RegisterAgentVersion(name, version string, fn AgentFunc)
func (s *Server) SetDefaultAgentVersion(name, version string) error
func (s *Server) Accept(ctx context.Context, t transport.Transport) error
func (s *Server) Close(ctx context.Context) error

type AgentFunc func(ctx context.Context, input json.RawMessage, jc *JobContext) (any, error)

type Session struct { ... }
func (s *Session) ID() string
func (s *Session) Principal() string
func (s *Session) NegotiatedFeatures() []string
func (s *Session) HasFeature(name string) bool

type Job struct { ... }
func (j *Job) ID() string
func (j *Job) Agent() string                           // "name@version"
func (j *Job) Lease() arcp.Lease

type JobContext struct { ... }
func (jc *JobContext) Log(level slog.Level, msg string, attrs ...slog.Attr)
func (jc *JobContext) Thought(text string)
func (jc *JobContext) ToolCall(tool string, args any) (callID string)
func (jc *JobContext) ToolResult(callID string, result any, err error)
func (jc *JobContext) Status(phase, msg string)
func (jc *JobContext) Metric(name string, value float64, unit string, dims map[string]string)
func (jc *JobContext) ArtifactRef(uri, contentType string, opts ArtifactOpts)
func (jc *JobContext) Progress(current uint64, opts ProgressOpts)
func (jc *JobContext) StreamResult(opts ResultStreamOpts) ResultWriter
func (jc *JobContext) Delegate(ctx context.Context, req DelegateRequest) (*JobHandle, error)
func (jc *JobContext) Budget() map[arcp.Currency]float64
func (jc *JobContext) Lease() arcp.Lease

type ResultWriter interface {
    io.WriteCloser                                     // Close finalizes; subsequent job.result carries result_id
}
```

`AgentFunc` returns `(any, error)`; the runtime marshals the `any`
into `job.result.payload.output` (or, if `StreamResult` was used,
the result is empty and `result_id` references the chunked stream
per §8.4 "Implementations MUST NOT mix inline result and
`result_chunk` in the same job").

`Session` and `Job` are accessors only — no setters. Mutation
happens through `JobContext` methods, which write to the session
outbox under the dedicated-writer goroutine described above.

### `auth/`

```
type Verifier interface {
    Verify(ctx context.Context, token string) (principal string, err error)
}

type BearerVerifier struct { ... }
```

(Survives from `auth/bearer.go`; rename and trim per
`02-current-audit.md` "Salvageable assets".)

## Hard rules

1. **No `init()` side effects** except `arcp.RegisterMessageType`
   calls in `messages/*.go`. Justified in §Type model: the registry
   key is the struct's `ARCPType()` method, and the registration
   call sits next to the struct that owns the key. Pattern matches
   `image.RegisterFormat` and `database/sql.Register`. Document
   the exception in the package doc on `arcp/messages/doc.go`.
2. **No global state** except the message-type registry (package
   `arcp`, package-level `map[string]reflect.Type`). The registry
   is only mutated during `init()`; reads after `main` starts are
   lock-free. Every other piece of state hangs off a `*Client`,
   `*Server`, `*Session`, or `*Job`.
3. **No panics in library code** except documented programmer-error
   preconditions. The only sanctioned panic is
   `arcp.RegisterMessageType` on duplicate type — same pattern as
   the current `arcp.RegisterMessageType` panic, retained. Agent
   panics are recovered at the `server.runJob` boundary
   (`defer recover()`) and converted to `job.error` with
   `CodeInternalError`; the spec doesn't require this but the Go
   idiom does.
4. **Every public function and type has a doc comment beginning
   with its name.** `go vet -all` and `revive` enforce. Example:
   `// Submit submits a job and returns a handle for tracking its
   lifecycle.`
5. **No `Get`-prefixed getters.** `Session.ID()`, not `GetID()`.
   `Client.Welcome()`, not `GetWelcome()`. Standard Go style; the
   current SDK already follows this.
6. **`errors.Is` / `errors.As` work across the sentinel layer.** A
   wrapped `*arcp.Error` with `Code: CodeLeaseExpired` must match
   the package-level `var ErrLeaseExpired = &Error{Code: CodeLeaseExpired}`
   sentinel via `errors.Is`. Implementation: `(*Error).Is` compares
   by `Code` only; `(*Error).Unwrap` returns the cause for `As`.

## What this leaves to later phases

- The byte-level wire encoding of the envelope (Phase 5,
  `05-wire.md`).
- The shape of `internal/eventlog/` against the v1.1 envelope and
  the resume buffer policy (Phase 5).
- Middleware adapters for `chi`, `gorilla/mux`, and OTel (Phase 5,
  `06-middleware.md`). The `middleware/otel` package consumes
  `go.opentelemetry.io/otel` only — no SDK choice imposed.
- The test matrix, including the goroutine-leak detector for the
  subscribe fan-out path (Phase 7, `07-tests.md`).

The combination of dedicated writer goroutine, channel-owning
subscription with `ctx`-guarded send, and `context.Context` on
every public method is the load-bearing concurrency design.
Everything else — feature negotiation, lease validation, budget
counters — is straight-line code wrapped around that core.
