# 05 — Middleware / Host Adapter Selection

Scope: pick the Go host-adapter sub-packages that mirror
`../typescript-sdk/packages/middleware/{node,express,fastify,hono,bun,otel}`,
and reject the ones whose port would be a pass-through with no Go
surface to add. Layout target (per 04-architecture.md):
`go-sdk/middleware/<name>/`, each a separately-importable Go module
or sub-package so callers do not transitively pull e.g. chi when
they only want `net/http`. WS engine is assumed to be
`coder/websocket` (Phase 3); fasthttp-side engine is assumed to be
absent from v1.1 (Phase 3 has not selected one).

## Adapter list

| Adapter   | Status                 | TS parity package          | Go API sketch                                                     | Go-specific seam                                                                                                            |
| --------- | ---------------------- | -------------------------- | ----------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------- |
| `nethttp` | REQUIRED               | `@arcp/node`               | `nethttp.NewHandler(s, opts) http.Handler`                        | `http.Handler` is the Go std-lib root interface; every other adapter composes through it                                    |
| `chi`     | REQUIRED               | (none — TS bundles router) | `chi.Mount(r, s, opts)`                                           | `chi.Router` users expect `r.Mount("/arcp", h)`; gorilla/mux is archived, chi is the actively-maintained `http.Handler` mux |
| `otel`    | REQUIRED               | `@arcp/middleware-otel`    | `otel.WrapServer(s, opts) *arcp.Server`                           | Bridges `go.opentelemetry.io/otel` `propagation.TextMapPropagator` to ARCP envelope `extensions["x-vendor.opentelemetry.tracecontext"]` |
| `gin`     | REJECT (defer to v1.2) | (no TS analogue)           | n/a                                                               | Pass-through: gin gives `c.Request`/`c.Writer.(http.Hijacker)`; same upgrade as `nethttp`                                   |
| `echo`    | REJECT (defer to v1.2) | (no TS analogue)           | n/a                                                               | Pass-through: `c.Request()`, `c.Response().Writer` is an `http.ResponseWriter`                                              |
| `fiber`   | REJECT (defer to v1.2) | (no TS analogue)           | n/a                                                               | fiber rides fasthttp; would need a fasthttp-native WS engine, which Phase 3 has not selected                                |

Section §14 (security) of `../spec/docs/draft-arcp-02.1.md`
requires DNS-rebind protection on every WS-accepting transport.
Every adapter below ships the same `allowedHosts []string` option,
default `["localhost", "127.0.0.1", "[::1]"]`, evaluated before the
WS upgrade returns `101`. Mismatch is rejected with HTTP `421
Misdirected Request` (RFC 7540 §9.1.2 semantics — request reached
a host that will not serve it). The TS adapters use `403`; the Go
SDK chooses `421` because the request is syntactically valid but
addressed to a host outside the allow-list, which is what `421`
exists to describe. Adapter docs will note this divergence.

## `nethttp` (required)

API sketch:

```go
package nethttp

type Options struct {
    Path         string        // default "/arcp"
    AllowedHosts []string      // default {"localhost","127.0.0.1","[::1]"}
    ReadLimit    int64         // default 1 << 20 (1 MiB), matches §14 chunk cap
    PingInterval time.Duration // WS-level keepalive; ARCP §6.4 ping is separate
}

func NewHandler(s *arcp.Server, opts Options) http.Handler
```

Mounting pattern:

```go
mux := http.NewServeMux()
mux.Handle("/arcp", nethttp.NewHandler(server, nethttp.Options{
    AllowedHosts: []string{"localhost"},
}))
srv := &http.Server{Addr: ":7777", Handler: mux}
```

Behavior:

1. Returned `http.Handler` rejects non-`GET` and missing
   `Upgrade: websocket` with `400`. Spec §4.
2. Host-header check runs before any WS handshake bytes — checked
   on `r.Host` with port stripped via `net.SplitHostPort`. Spec
   §14 ("Lease expiration clock", "Heartbeat amplification", and
   the v1.0 §14.x DNS-rebind discussion).
3. WS upgrade calls `websocket.Accept` from `coder/websocket`; the
   accepted conn is wrapped in `arcp.Transport` and handed to
   `server.Accept(ctx, transport)`.

Shutdown deadlock — name the bug:

`(*http.Server).Shutdown(ctx)` in the standard library waits for
all idle connections to drain and for all in-flight handlers to
return. **A WS connection upgraded via `http.Hijacker` is not in
either bucket**: `net/http` releases the hijacked conn from its
tracking before the handler returns, so `Shutdown` returns
immediately even while WS streams remain open. The adapter MUST
track active conns itself:

- `NewHandler` returns a value that also satisfies an unexported
  `closeAll(context.Context) error` method.
- The exported `Shutdown(ctx)` on the adapter is the documented
  call to use; integrators wire it as
  `srv.RegisterOnShutdown(handler.Shutdown)`.
- Internally, each accepted conn registers in a `sync.Map` keyed
  by a counter; `Shutdown` writes ARCP `session.bye` (§6.7) with a
  drain reason on every entry, then closes the WS with status
  `1001` (Going Away) after the ctx deadline.

Read/write deadlines:

- Per-frame read deadline is set by the WS engine
  (`coder/websocket` exposes `Conn.SetReadLimit`).
- ARCP heartbeats (§6.4 `session.ping` / `session.pong`) live at
  the protocol layer; they are not coupled to WS-level pings. The
  adapter does NOT emit WS pings; that is left to the engine
  defaults so a v1.0 client (which does not implement §6.4) still
  stays connected via WS pong.

## `chi` (required)

Why `go-chi/chi` over `gorilla/mux`: `gorilla/mux` was archived in
December 2022; the Gorilla toolkit was un-archived in 2023 as
"community-maintained" with no release manager assigned, and
`github.com/gorilla/mux` releases lag behind. `go-chi/chi/v5` is
actively released, is a stricter `http.Handler` composition (no
custom `Handler` interface, no reflection), and is the de-facto
choice for new Go HTTP services written against the standard
library shape. The Go SDK adapter wraps that idiom; the choice is
not "chi is better" but "chi is what current Go HTTP code uses,
and gorilla/mux requires a separate adapter the SDK does not want
to maintain".

API sketch:

```go
package chi

import (
    chir "github.com/go-chi/chi/v5"
    "github.com/agentruntimecontrolprotocol/go-sdk/middleware/nethttp"
)

type Options = nethttp.Options // alias, no new fields

func Mount(r chir.Router, s *arcp.Server, opts Options)
```

Implementation is one line of glue: `r.Handle(opts.Path,
nethttp.NewHandler(s, opts))`. Defense for a separate sub-package:

1. **Import-graph isolation.** Users of `nethttp` do not transitively
   import `go-chi/chi/v5`. v1.1 ships them as separate Go modules
   (`middleware/nethttp/go.mod`, `middleware/chi/go.mod`) so the
   chi dependency is pay-for-what-you-use.
2. **Mount idiom.** chi users expect `r.Mount` / `r.Handle` calls
   on a router value, not "pull out the underlying mux and call
   `Handle` yourself". Surface that idiom in one well-named function.
3. **Future divergence.** When the `Server` exposes status / metric
   endpoints (HTTP, not WS), the chi adapter is the place that
   will register them per chi's middleware-stack conventions
   (`r.Get("/arcp/status", ...)`). That is not in v1.1, but the
   sub-package is the seam for it.

## `otel` (required)

Parity targets with `@arcp/middleware-otel`
(`../typescript-sdk/packages/middleware/otel/src/index.ts`):

**1. Trace context bridge.** The TS adapter does **not** put the
W3C traceparent in `envelope.trace_id`. `trace_id` per spec §11 is
the 16-byte (32-hex) trace identifier alone; the full traceparent
+ tracestate carrier lives in the envelope's `extensions`
namespace under the §15-mandated key
`x-vendor.opentelemetry.tracecontext`. The Go adapter mirrors this
exactly:

- On send, `otel.GetTextMapPropagator().Inject(ctx, carrier)` into a
  `propagation.MapCarrier`; the resulting `traceparent`,
  `tracestate` strings go into `env.Extensions["x-vendor.opentelemetry.tracecontext"]`.
  Additionally, the active span's `TraceID().String()` (lowercase
  32-hex) is written into `env.TraceID` for §11 conformance.
- On recv, the inverse: extract from
  `env.Extensions["x-vendor.opentelemetry.tracecontext"]` into the
  parent context, then `tracer.Start(parent, "arcp.recv "+env.Type, ...)`.
- If extensions has no OTel key but `env.TraceID` is set, the
  adapter starts a new root span using that trace ID (so a v1.0
  peer that fills `trace_id` but not the extensions key still
  gives a correlatable span).

**2. Span granularity.** Two-tier default mirroring TS:

| Layer    | Span                                              | Default                |
| -------- | ------------------------------------------------- | ---------------------- |
| Per-frame | `arcp.send <type>` / `arcp.recv <type>`           | OFF (high cardinality) |
| Per-job   | `arcp.job <agent>` from `job.submit` → terminal   | ON                     |
| Per-tool_call | `arcp.tool_call <tool_name>` inside a job event | ON                  |

Controlled by `otel.Options{ FrameSpans, JobSpans, ToolCallSpans bool }`.
TS exposes only per-frame; the Go adapter exposes both because
per-frame spans in a busy ARCP runtime (heartbeats, acks, progress
events) blow up trace cardinality. Per-job + per-tool_call is the
useful unit and is on by default.

**3. v1.1 attributes** (spec §11):

| Attribute                  | Type          | Source                                                          |
| -------------------------- | ------------- | --------------------------------------------------------------- |
| `arcp.session_id`          | string        | `env.SessionID`                                                 |
| `arcp.job_id`              | string        | `env.JobID`                                                     |
| `arcp.trace_id`            | string        | `env.TraceID`                                                   |
| `arcp.agent`               | string        | `job.submit.payload.agent`                                      |
| `arcp.lease.capabilities`  | string (csv)  | sorted keys of `payload.lease` / `payload.lease_request`        |
| `arcp.lease.expires_at`    | string (RFC 3339) | `payload.lease_constraints.expires_at` _(v1.1)_             |
| `arcp.budget.remaining`    | string (JSON) | `json.Marshal(jobCtx.Budget().Snapshot())` — `map[string]float64` |

Attribute names match TS character-for-character. The Go adapter
emits both attribute names verbatim so a single Tempo/Honeycomb
dashboard query works for either runtime.

**4. Server interceptor — chosen shape.**

```go
package otel

type Options struct {
    TracerProvider trace.TracerProvider // default otel.GetTracerProvider()
    Propagator     propagation.TextMapPropagator // default otel.GetTextMapPropagator()
    FrameSpans     bool                 // default false
    JobSpans       bool                 // default true
    ToolCallSpans  bool                 // default true
}

func WrapServer(s *arcp.Server, opts Options) *arcp.Server
func WrapClient(c *arcp.Client, opts Options) *arcp.Client
```

Chosen over a hook-registration shape (e.g.
`s.OnSend(otel.SendHook(opts))`) for two reasons:

1. **Single point of bridging.** `WrapServer` returns a wrapped
   value whose `Accept(transport)` method wraps the transport in
   `transport.Tracing(inner, tp, prop, opts)`. That is the same
   seam the TS package uses (`withTracing(transport, opts)`); both
   ports sit on the transport, not on the protocol handlers. The
   protocol handlers do not need to know OTel exists.
2. **No partial wiring.** A hook-registration API lets a user
   forget to register one of the four hooks (send, recv, job-start,
   job-end) and get half-instrumented traces. `WrapServer` either
   instruments everything or returns an explicit error.

The wrapped value is `*arcp.Server` (not a new type) by composing
through internal interface-typed fields, so existing user code
continues to compile.

## `gin`, `echo`, `fiber` (defensible adds — call)

**`gin` — REJECT for v1.1.** gin gives you `c.Request *http.Request`
and `c.Writer gin.ResponseWriter`, the latter embedding
`http.ResponseWriter`. The WS upgrade path is identical to
`nethttp.NewHandler`: pull `c.Request` and `c.Writer`, call
`websocket.Accept`. There is no gin-specific seam. The "gin
adapter" would be five lines:

```go
func Handler(s *arcp.Server, opts Options) gin.HandlerFunc {
    h := nethttp.NewHandler(s, opts)
    return func(c *gin.Context) { h.ServeHTTP(c.Writer, c.Request) }
}
```

That is documentation in the README, not a sub-package. Deferred
until v1.2, criterion to unblock: gin ships a non-`http.Handler`
hook the SDK needs to special-case (e.g. a custom HTTP/2 push
handler ARCP wants to ride on top of).

**`echo` — REJECT for v1.1.** Same argument as gin: `c.Request()`
returns `*http.Request`, `c.Response().Writer` is an
`http.ResponseWriter`. Pass-through. Same five-line README
snippet. Deferred until v1.2, criterion: echo's middleware
ordering becomes load-bearing for the SDK (e.g. echo-specific
context-propagation that `WrapServer` cannot do externally).

**`fiber` — REJECT for v1.1.** fiber rides on `fasthttp`;
`c.Context()` returns `*fasthttp.RequestCtx`, which is **not**
`http.ResponseWriter`. WS upgrade has to go through
`gofiber/contrib/websocket` (which wraps `fasthttp/websocket`), a
different engine from `coder/websocket`. That is a real
Go-specific seam — fiber users cannot pass-through to `nethttp`.

But: Phase 3 selected `coder/websocket` as the SDK's only WS
engine. Shipping fiber would require maintaining a second WS
binding inside the SDK (envelope codec + read/write loop +
heartbeat watchdog), against a fasthttp WS implementation whose
maintainership and security history differ from `coder/websocket`.
For v1.1 that is not justified by user demand we can name.

Deferred until v1.2, criterion: fiber/fasthttp WS engine is added
to the supported set under the same conformance test suite as
`coder/websocket`, OR fiber adds a `c.Hijack() (net.Conn, error)`
that surfaces a standard `net.Conn` the SDK's normal upgrade path
can ride.

## Why no `hono` or `bun`

`@arcp/middleware-hono` exists because Hono is a JS-runtime-portable
HTTP router (Workers, Deno, Node, Bun). Go has no analogue: chi,
gin, echo all assume `net/http` and a single runtime. There is no
"portable Go HTTP router" because Go's runtime story is uniform.
Skipped, no replacement.

`@arcp/middleware-bun` exists because Bun is a separate JS runtime
with its own `Bun.serve` API. Go has one runtime; there is no
equivalent split to bridge. Skipped, no replacement.

## Coverage matrix — TS → Go

| TS package                | Go adapter                | Parity   | Rationale                                                                                       |
| ------------------------- | ------------------------- | -------- | ----------------------------------------------------------------------------------------------- |
| `@arcp/node`              | `middleware/nethttp`      | full     | Both are the "raw HTTP server upgrade" seam; Node has `http.Server`, Go has `*http.Server`      |
| `@arcp/express`           | `middleware/nethttp`      | full     | Express is a Node-`http.Server`-compatible layer; Go's `net/http` is the equivalent baseline    |
| `@arcp/fastify`           | `middleware/nethttp`      | full     | Fastify exposes `app.server` (Node `http.Server`); same handoff as Express                       |
| `@arcp/hono`              | `middleware/nethttp`      | full     | Hono via `@hono/node-server` returns a Node `http.Server`; same handoff                          |
| (none, chi)               | `middleware/chi`          | full     | Go-specific router idiom; one-line wrapper over `nethttp` but separate sub-package per import-graph rule above |
| `@arcp/bun`               | none                      | skip     | Bun is a JS runtime; no Go equivalent                                                            |
| `@arcp/middleware-otel`   | `middleware/otel`         | full     | OTel API is cross-language; attribute names match; carrier in `extensions["x-vendor.opentelemetry.tracecontext"]` |
| (none, gin)               | none (v1.1)               | deferred | Pass-through; no Go seam beyond `nethttp`                                                        |
| (none, echo)              | none (v1.1)               | deferred | Pass-through; no Go seam beyond `nethttp`                                                        |
| (none, fiber)             | none (v1.1)               | deferred | Would require a second WS engine (fasthttp-side); Phase 3 picked one engine                      |

Three required, three deferred, two structurally-absent. The v1.1
Go SDK ships three adapter sub-packages: `middleware/nethttp`,
`middleware/chi`, `middleware/otel`.
