# Transports

ARCP is transport neutral. This SDK ships three implementations of
`transport.Transport`.

| Transport | Use when |
| --- | --- |
| WebSocket | Client and runtime are separate processes or hosts. |
| stdio | A supervisor launches an agent/runtime process and speaks NDJSON. |
| in-memory | Tests, examples, and same-process embedding. |

The `Transport` interface has three methods: `Send(ctx, env)`,
`Recv(ctx)`, and `Close()`. Implementations guarantee at most one
goroutine calls `Send` and one calls `Recv` at a time; both client and
server respect that contract. `transport.ErrClosed` is the sentinel
returned by `Recv` after `Close` has been called.

## WebSocket

Server side, mount the runtime on `net/http`:

```go
import (
    arcpnethttp "github.com/agentruntimecontrolprotocol/go-sdk/middleware/nethttp"
    "github.com/agentruntimecontrolprotocol/go-sdk/server"
)

srv := server.New(server.Options{})
http.Handle("/arcp", arcpnethttp.NewHandler(srv, arcpnethttp.Options{}))
```

Or on a `chi.Router` via the chi sub-package (which exposes a `Mount`
helper, not a separate constructor):

```go
import (
    arcpchi "github.com/agentruntimecontrolprotocol/go-sdk/middleware/chi"
    "github.com/agentruntimecontrolprotocol/go-sdk/server"
    "github.com/go-chi/chi/v5"
)

srv := server.New(server.Options{})
r := chi.NewRouter()
arcpchi.Mount(r, srv, arcpchi.Options{})
```

Both return a `*nethttp.Handler` you can use for `Shutdown`. The
handler defaults `AllowedHosts` to the loopback set (`localhost`,
`127.0.0.1`, `[::1]`) per spec §14 DNS-rebind protection, sets the
inbound frame `ReadLimit` to 1 MiB, and requires `GET` for the
upgrade. To go beyond loopback set `Options.AllowedHosts` explicitly.

Dial from the client side:

```go
t, err := transport.DialWebSocket(ctx, "ws://localhost:7777/arcp", transport.WebSocketOptions{})
```

`WebSocketOptions` exposes `Subprotocols`, `HTTPHeader`, `HTTPClient`,
and `ReadLimit` (1 MiB default). Pass `transport.NewWebSocket(conn)`
if you already have a `*websocket.Conn` from `coder/websocket` (e.g.
when integrating with a custom HTTP upgrade path).

The handler's `PingInterval` option drives WebSocket-layer pings to
keep idle TCP connections alive through NAT/load balancer timeouts;
this is independent of ARCP's `session.ping` heartbeat.

## stdio

`transport.NewStdioTransport(in io.Reader, out io.Writer)` returns a
transport that reads and writes one JSON envelope per line. It is
useful for child runtimes a supervisor launches, sandboxed agents, or
piping `arcp submit` into a long-running process.

## in-memory

`transport.NewMemoryPair()` returns two connected endpoints whose
sends become the other's recvs. The integration tests use it to
exercise the full client/server protocol without a network listener;
embedders can use it to run both sides in the same process.
