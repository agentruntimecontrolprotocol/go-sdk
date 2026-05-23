# Package `transport`

Import path: `github.com/agentruntimecontrolprotocol/go-sdk/transport`

`transport.Transport` is the envelope send/receive abstraction used
by both client and server.

## Interface

```go
type Transport interface {
    Send(ctx context.Context, env arcp.Envelope) error
    Recv(ctx context.Context) (arcp.Envelope, error)
    Close() error
}

var ErrClosed = errors.New("transport: closed")
```

Implementations must allow one concurrent `Send` caller and one
concurrent `Recv` caller; both client and server respect that
contract. `Close` cancels any in-flight `Recv` and causes subsequent
calls to return `ErrClosed`.

## Built-in implementations

| Function | Purpose |
| --- | --- |
| `DialWebSocket(ctx, url, WebSocketOptions)` | Client-side WebSocket dial via `coder/websocket`. |
| `NewWebSocket(conn)` | Wrap an already-upgraded `*websocket.Conn`. |
| `NewStdioTransport(in, out)` | NDJSON envelope transport over `io.Reader` and `io.Writer` — one JSON envelope per line. |
| `NewMemoryPair()` | Two connected in-process transports for tests and embedding. |

### `WebSocketOptions`

| Field | Default | Notes |
| --- | --- | --- |
| `Subprotocols` | none | Negotiated subprotocols. |
| `HTTPHeader` | none | Extra headers on the upgrade request. |
| `HTTPClient` | `http.DefaultClient` | For custom transports / proxies. |
| `ReadLimit` | `1 MiB` | Frame-size cap, must match server. |

See [transports.md](../transports.md) for end-to-end usage.

Custom transports only need to implement the three interface methods.
